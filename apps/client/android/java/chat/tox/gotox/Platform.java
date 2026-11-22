package chat.tox.gotox;

import android.Manifest;
import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.view.View;
import org.gioui.GioActivity;

/** What the Go code calls on Android. */
public final class Platform {
	static final String CHANNEL_MESSAGES = "messages";
	static final String CHANNEL_SERVICE = "service";

	private Platform() {}

	/**
	 * stayConnected runs the service that keeps Tox connected while the app is
	 * closed, or stops it. The boot receiver starts it again after a restart.
	 */
	public static void stayConnected(Context ctx, boolean on) {
		prefs(ctx).edit().putBoolean("stay_connected", on).apply();
		Intent service = new Intent(ctx, ToxService.class);
		if (on) {
			ctx.startForegroundService(service);
		} else {
			ctx.stopService(service);
		}
	}

	static boolean staysConnected(Context ctx) {
		return prefs(ctx).getBoolean("stay_connected", false);
	}

	private static SharedPreferences prefs(Context ctx) {
		return ctx.getSharedPreferences("platform", Context.MODE_PRIVATE);
	}

	/** askForNotifications asks the user to allow notifications, if they haven't. */
	public static void askForNotifications(View view) {
		Activity a = (Activity) view.getContext();
		if (a.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
			return;
		}
		a.runOnUiThread(() -> a.requestPermissions(new String[] {Manifest.permission.POST_NOTIFICATIONS}, 1));
	}

	/** askForCall asks the user to let the app record sound, and with video the camera. */
	public static void askForCall(View view, boolean video) {
		Activity a = (Activity) view.getContext();
		String[] want = video
			? new String[] {Manifest.permission.RECORD_AUDIO, Manifest.permission.CAMERA}
			: new String[] {Manifest.permission.RECORD_AUDIO};
		for (String p : want) {
			if (a.checkSelfPermission(p) != PackageManager.PERMISSION_GRANTED) {
				a.runOnUiThread(() -> a.requestPermissions(want, 2));
				return;
			}
		}
	}

	/**
	 * notify shows a message notification. There is one for each tag: a new
	 * one replaces it. Tapping it opens uri in the app.
	 */
	public static void notify(Context ctx, String tag, String title, String text, String uri) {
		createChannels(ctx);
		Notification n = new Notification.Builder(ctx, CHANNEL_MESSAGES)
			.setSmallIcon(R.drawable.ic_notification)
			.setContentTitle(title)
			.setContentText(text)
			.setStyle(new Notification.BigTextStyle().bigText(text))
			.setCategory(Notification.CATEGORY_MESSAGE)
			.setContentIntent(open(ctx, uri))
			.setAutoCancel(true)
			.build();
		ctx.getSystemService(NotificationManager.class).notify(tag, 0, n);
	}

	public static void cancel(Context ctx, String tag) {
		ctx.getSystemService(NotificationManager.class).cancel(tag, 0);
	}

	/** open is an intent to open the app, at uri if it isn't null. */
	static PendingIntent open(Context ctx, String uri) {
		Intent intent = new Intent(ctx, GioActivity.class);
		if (uri != null) {
			intent.setAction(Intent.ACTION_VIEW).setData(Uri.parse(uri));
		}
		int code = uri == null ? 0 : uri.hashCode();
		return PendingIntent.getActivity(ctx, code, intent,
			PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
	}

	static void createChannels(Context ctx) {
		NotificationManager m = ctx.getSystemService(NotificationManager.class);
		m.createNotificationChannel(new NotificationChannel(
			CHANNEL_MESSAGES, "Messages", NotificationManager.IMPORTANCE_HIGH));
		NotificationChannel service = new NotificationChannel(
			CHANNEL_SERVICE, "Staying connected", NotificationManager.IMPORTANCE_MIN);
		service.setShowBadge(false);
		m.createNotificationChannel(service);
	}
}
