package chat.tox.gotox;

import android.Manifest;
import android.content.Context;
import android.content.pm.PackageManager;
import android.graphics.ImageFormat;
import android.hardware.camera2.CameraAccessException;
import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCharacteristics;
import android.hardware.camera2.CameraDevice;
import android.hardware.camera2.CameraManager;
import android.hardware.camera2.CaptureRequest;
import android.media.AudioAttributes;
import android.media.AudioFormat;
import android.media.AudioManager;
import android.media.AudioRecord;
import android.media.AudioTrack;
import android.media.Image;
import android.media.ImageReader;
import android.media.MediaRecorder;
import android.os.Handler;
import android.os.HandlerThread;
import android.util.Log;
import android.view.Surface;
import java.nio.ByteBuffer;
import java.util.Collections;

/**
 * Sound and the camera, for calls. Go starts and stops them; what they record
 * goes to Go through the native methods.
 */
public final class Media {
	private static final String TAG = "GoTox";
	private static final int RATE = 48000;
	private static final int FRAME = RATE / 50;

	private Media() {}

	/** recorded is 20 ms of sound, 16-bit samples in the CPU's byte order. */
	static native void recorded(byte[] pcm);

	static native void captured(int width, int height, byte[] y, byte[] u, byte[] v);

	// Sound.

	private static Thread recorder;
	private static volatile boolean recording;
	private static AudioTrack track;
	private static int trackRate, trackChannels;

	/** startAudio records until stopAudio, once the user lets the app. */
	public static synchronized void startAudio(Context ctx) {
		if (recording) {
			return;
		}
		recording = true;
		AudioManager am = ctx.getSystemService(AudioManager.class);
		am.setMode(AudioManager.MODE_IN_COMMUNICATION);
		am.setSpeakerphoneOn(true);
		recorder = new Thread(() -> record(ctx), "Go Tox microphone");
		recorder.start();
	}

	private static void record(Context ctx) {
		// The user may still be answering the question.
		for (int i = 0; recording && ctx.checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED; i++) {
			if (i == 120) {
				Log.w(TAG, "no permission to record");
				return;
			}
			try {
				Thread.sleep(500);
			} catch (InterruptedException e) {
				return;
			}
		}
		int min = AudioRecord.getMinBufferSize(RATE, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT);
		AudioRecord rec;
		try {
			rec = new AudioRecord(MediaRecorder.AudioSource.VOICE_COMMUNICATION, RATE,
				AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT, Math.max(min, FRAME * 2 * 4));
		} catch (SecurityException e) {
			Log.w(TAG, "no permission to record", e);
			return;
		}
		rec.startRecording();
		byte[] pcm = new byte[FRAME * 2];
		while (recording) {
			int n = 0;
			while (n < pcm.length && recording) {
				int got = rec.read(pcm, n, pcm.length - n);
				if (got <= 0) {
					break;
				}
				n += got;
			}
			if (n == pcm.length) {
				recorded(pcm);
			}
		}
		rec.stop();
		rec.release();
	}

	/** play plays a frame of the call's sound: 16-bit samples, as recorded. */
	public static synchronized void play(byte[] pcm, int channels, int rate) {
		if (track == null || trackRate != rate || trackChannels != channels) {
			if (track != null) {
				track.release();
			}
			int mask = channels == 2 ? AudioFormat.CHANNEL_OUT_STEREO : AudioFormat.CHANNEL_OUT_MONO;
			int min = AudioTrack.getMinBufferSize(rate, mask, AudioFormat.ENCODING_PCM_16BIT);
			track = new AudioTrack.Builder()
				.setAudioAttributes(new AudioAttributes.Builder()
					.setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION)
					.setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
					.build())
				.setAudioFormat(new AudioFormat.Builder()
					.setSampleRate(rate)
					.setChannelMask(mask)
					.setEncoding(AudioFormat.ENCODING_PCM_16BIT)
					.build())
				.setBufferSizeInBytes(Math.max(min, rate / 50 * channels * 2 * 4))
				.setTransferMode(AudioTrack.MODE_STREAM)
				.build();
			track.play();
			trackRate = rate;
			trackChannels = channels;
		}
		track.write(pcm, 0, pcm.length, AudioTrack.WRITE_NON_BLOCKING);
	}

	public static synchronized void stopAudio(Context ctx) {
		recording = false;
		if (track != null) {
			track.release();
			track = null;
		}
		ctx.getSystemService(AudioManager.class).setMode(AudioManager.MODE_NORMAL);
	}

	// The camera.

	private static HandlerThread cameraThread;
	private static CameraDevice camera;
	private static CameraCaptureSession session;
	private static ImageReader reader;
	private static int rotation;

	/**
	 * startCamera sends the front camera's pictures until stopCamera, once the
	 * user lets the app. It says whether there is a camera.
	 */
	public static synchronized boolean startCamera(Context ctx) {
		if (ctx.checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
			return openCamera(ctx);
		}
		// The user may still be answering the question.
		cameraWanted = true;
		new Thread(() -> {
			for (int i = 0; i < 120; i++) {
				try {
					Thread.sleep(500);
				} catch (InterruptedException e) {
					return;
				}
				synchronized (Media.class) {
					if (!cameraWanted) {
						return;
					}
					if (ctx.checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
						openCamera(ctx);
						return;
					}
				}
			}
		}, "Go Tox camera permission").start();
		return true;
	}

	private static boolean cameraWanted;

	private static boolean openCamera(Context ctx) {
		cameraWanted = false;
		CameraManager cm = ctx.getSystemService(CameraManager.class);
		try {
			String id = null;
			for (String c : cm.getCameraIdList()) {
				CameraCharacteristics ch = cm.getCameraCharacteristics(c);
				Integer facing = ch.get(CameraCharacteristics.LENS_FACING);
				if (id == null || (facing != null && facing == CameraCharacteristics.LENS_FACING_FRONT)) {
					id = c;
					Integer o = ch.get(CameraCharacteristics.SENSOR_ORIENTATION);
					rotation = o == null ? 0 : o;
				}
			}
			if (id == null) {
				return false;
			}
			cameraThread = new HandlerThread("Go Tox camera");
			cameraThread.start();
			Handler handler = new Handler(cameraThread.getLooper());
			reader = ImageReader.newInstance(640, 480, ImageFormat.YUV_420_888, 2);
			reader.setOnImageAvailableListener(r -> {
				Image img = r.acquireLatestImage();
				if (img != null) {
					send(img);
					img.close();
				}
			}, handler);
			cm.openCamera(id, new CameraDevice.StateCallback() {
				@Override public void onOpened(CameraDevice d) {
					synchronized (Media.class) {
						camera = d;
						capture(handler);
					}
				}

				@Override public void onDisconnected(CameraDevice d) {
					d.close();
				}

				@Override public void onError(CameraDevice d, int error) {
					Log.w(TAG, "camera error " + error);
					d.close();
				}
			}, handler);
			return true;
		} catch (CameraAccessException | SecurityException e) {
			Log.w(TAG, "no camera", e);
			return false;
		}
	}

	private static void capture(Handler handler) {
		try {
			Surface surface = reader.getSurface();
			CaptureRequest.Builder req = camera.createCaptureRequest(CameraDevice.TEMPLATE_RECORD);
			req.addTarget(surface);
			camera.createCaptureSession(Collections.singletonList(surface), new CameraCaptureSession.StateCallback() {
				@Override public void onConfigured(CameraCaptureSession s) {
					synchronized (Media.class) {
						session = s;
						try {
							s.setRepeatingRequest(req.build(), null, handler);
						} catch (CameraAccessException e) {
							Log.w(TAG, "camera", e);
						}
					}
				}

				@Override public void onConfigureFailed(CameraCaptureSession s) {
					Log.w(TAG, "camera session failed");
				}
			}, handler);
		} catch (CameraAccessException e) {
			Log.w(TAG, "camera", e);
		}
	}

	/**
	 * The camera thread's buffers: three planes, then the same turned. Go copies
	 * a picture before send returns, so the next one reuses them.
	 */
	private static final byte[][] buffers = new byte[6][];

	private static byte[] buffer(int i, int size) {
		if (buffers[i] == null || buffers[i].length != size) {
			buffers[i] = new byte[size];
		}
		return buffers[i];
	}

	/** send hands Go a picture in I420, upright. */
	private static void send(Image img) {
		int w = img.getWidth(), h = img.getHeight();
		int cw = (w + 1) / 2, ch = (h + 1) / 2;
		Image.Plane[] p = img.getPlanes();
		byte[] y = plane(p[0], w, h, buffer(0, w * h));
		byte[] u = plane(p[1], cw, ch, buffer(1, cw * ch));
		byte[] v = plane(p[2], cw, ch, buffer(2, cw * ch));
		if (rotation != 90 && rotation != 270) {
			captured(w, h, y, u, v);
			return;
		}
		boolean ccw = rotation == 270;
		captured(h, w, rotate(y, w, h, ccw, buffer(3, w * h)), rotate(u, cw, ch, ccw, buffer(4, cw * ch)),
			rotate(v, cw, ch, ccw, buffer(5, cw * ch)));
	}

	/** plane copies a plane without its padding into out. */
	private static byte[] plane(Image.Plane p, int w, int h, byte[] out) {
		ByteBuffer buf = p.getBuffer();
		int row = p.getRowStride(), pixel = p.getPixelStride();
		if (pixel == 1 && row == w) {
			buf.get(out, 0, Math.min(out.length, buf.remaining()));
			return out;
		}
		for (int j = 0; j < h; j++) {
			for (int i = 0; i < w; i++) {
				int at = j * row + i * pixel;
				out[j * w + i] = at < buf.limit() ? buf.get(at) : 0;
			}
		}
		return out;
	}

	/** rotate turns a plane a quarter into out, clockwise, or anticlockwise with ccw. */
	private static byte[] rotate(byte[] in, int w, int h, boolean ccw, byte[] out) {
		for (int j = 0; j < h; j++) {
			for (int i = 0; i < w; i++) {
				int x = ccw ? j : h - 1 - j;
				int yy = ccw ? w - 1 - i : i;
				out[yy * h + x] = in[j * w + i];
			}
		}
		return out;
	}

	public static synchronized void stopCamera() {
		cameraWanted = false;
		if (session != null) {
			session.close();
			session = null;
		}
		if (camera != null) {
			camera.close();
			camera = null;
		}
		if (reader != null) {
			reader.close();
			reader = null;
		}
		if (cameraThread != null) {
			cameraThread.quitSafely();
			cameraThread = null;
		}
	}
}
