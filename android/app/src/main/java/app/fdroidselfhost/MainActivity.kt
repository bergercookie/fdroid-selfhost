package app.fdroidselfhost

import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.text.InputType
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

/**
 * Companion app for an fdroid-selfhost server: enter the server URL once, then
 * add its repository to the F-Droid app or open its web UI.
 */
class MainActivity : Activity() {
    private val prefs by lazy { getSharedPreferences("settings", MODE_PRIVATE) }
    private lateinit var urlField: EditText
    private lateinit var status: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        urlField = EditText(this).apply {
            hint = "https://fdroid.example.com"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            setSingleLine()
            setText(prefs.getString(KEY_URL, BuildConfig.DEFAULT_SERVER_URL))
        }
        status = TextView(this)

        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 48, 48, 48)
            addView(TextView(context).apply { text = "Server URL" })
            addView(urlField)
            addView(button("Add repository to F-Droid") { addToFdroid() })
            addView(button("Open web UI") { openWebUi() })
            addView(status)
        }
        setContentView(layout)
    }

    private fun button(label: String, onClick: () -> Unit) =
        Button(this).apply {
            text = label
            setOnClickListener { onClick() }
        }

    /** The URL the user typed, validated and remembered; null if unusable. */
    private fun serverUrl(): String? {
        val url = urlField.text.toString().trim().trimEnd('/')
        if (!url.startsWith("http://") && !url.startsWith("https://")) {
            status.text = "Enter a URL starting with http:// or https://"
            return null
        }
        prefs.edit().putString(KEY_URL, url).apply()
        return url
    }

    private fun openWebUi() {
        val url = serverUrl() ?: return
        open(Uri.parse(url))
    }

    private fun addToFdroid() {
        val url = serverUrl() ?: return
        status.text = "Contacting server…"
        thread {
            val result = runCatching { fetchFdroidLink(url) }
            runOnUiThread {
                result
                    .onSuccess { open(Uri.parse(it)) }
                    .onFailure { status.text = "Could not reach the server: ${it.message}" }
            }
        }
    }

    /** Asks the server for the fdroidrepo(s):// link that carries its signing fingerprint. */
    private fun fetchFdroidLink(serverUrl: String): String {
        val connection = URL("$serverUrl/api/info").openConnection() as HttpURLConnection
        connection.connectTimeout = TIMEOUT_MS
        connection.readTimeout = TIMEOUT_MS
        try {
            val body = connection.inputStream.bufferedReader().use { it.readText() }
            return JSONObject(body).getString("fdroidLink")
        } finally {
            connection.disconnect()
        }
    }

    private fun open(uri: Uri) {
        try {
            startActivity(Intent(Intent.ACTION_VIEW, uri))
            status.text = ""
        } catch (e: ActivityNotFoundException) {
            status.text = "No app can open this link. Is F-Droid installed?"
        }
    }

    private companion object {
        const val KEY_URL = "server_url"
        const val TIMEOUT_MS = 10_000
    }
}
