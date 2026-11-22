package chat.tox.gotox;

import android.app.ForegroundServiceStartNotAllowedException;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.util.Log;

/** BootReceiver connects again after the phone starts, if the user wants. */
public final class BootReceiver extends BroadcastReceiver {
	@Override public void onReceive(Context ctx, Intent intent) {
		if (!Intent.ACTION_BOOT_COMPLETED.equals(intent.getAction()) || !Platform.staysConnected(ctx)) {
			return;
		}
		try {
			ctx.startForegroundService(new Intent(ctx, ToxService.class));
		} catch (ForegroundServiceStartNotAllowedException e) {
			// Android decides: the app connects when the user opens it.
			Log.w("GoTox", "not connecting after boot", e);
		}
	}
}
