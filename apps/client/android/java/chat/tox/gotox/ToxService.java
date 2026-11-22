package chat.tox.gotox;

import android.app.Notification;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.IBinder;
import org.gioui.Gio;

/**
 * ToxService keeps the app running, and Tox connected, while the app is closed.
 * Tox runs in Go: the service only keeps the process alive.
 */
public final class ToxService extends Service {
	@Override public void onCreate() {
		super.onCreate();
		// After a reboot, or when Android restarts the service, the app isn't
		// running yet: this runs its Go main, which starts Tox.
		Gio.init(getApplicationContext());
	}

	@Override public int onStartCommand(Intent intent, int flags, int startId) {
		Platform.createChannels(this);
		Notification n = new Notification.Builder(this, Platform.CHANNEL_SERVICE)
			.setSmallIcon(R.drawable.ic_notification)
			.setContentTitle("Staying connected")
			.setContentText("Go Tox gets your messages while it's closed.")
			.setContentIntent(Platform.open(this, null))
			.setOngoing(true)
			.build();
		startForeground(1, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_REMOTE_MESSAGING);
		return START_STICKY;
	}

	@Override public IBinder onBind(Intent intent) {
		return null;
	}
}
