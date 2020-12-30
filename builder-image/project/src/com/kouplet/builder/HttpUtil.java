package com.kouplet.builder;

import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URLConnection;
import java.security.KeyManagementException;
import java.security.NoSuchAlgorithmException;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.HttpsURLConnection;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLSession;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509TrustManager;

public class HttpUtil {

	public static InputStream get(URL url) throws IOException {
		HttpURLConnection connection = null;

		connection = (HttpURLConnection) url.openConnection();

		connection.setRequestMethod("GET");

		allowAllCerts(connection);

		int responseCode = connection.getResponseCode();
		boolean isGoodResponse = responseCode > 199 && responseCode < 300;

		if (!isGoodResponse) {
			throw new IOException("Received bad response code " + responseCode + " from " + connection.getURL());
		}

		InputStream is = connection.getInputStream();
		return is;
	}

	private static void allowAllCerts(URLConnection connection) {
		if (connection instanceof HttpsURLConnection) {
			HttpsURLConnection huc = (HttpsURLConnection) connection;

			// Ignore invalid certificates since we're using internal sites
			X509TrustManager tm = new X509TrustManager() {
				public void checkClientTrusted(X509Certificate[] xcs, String str) throws CertificateException {
					// Do nothing
				}

				public void checkServerTrusted(X509Certificate[] xcs, String str) throws CertificateException {
					// Do nothing
				}

				public X509Certificate[] getAcceptedIssuers() {
					return null;
				}
			};

			// Don't bother to verify that hostname resolves correctly
			HostnameVerifier hostnameVerifier = new HostnameVerifier() {
				@Override
				public boolean verify(String hostname, SSLSession session) {
					return true;
				}
			};

			huc.setHostnameVerifier(hostnameVerifier);

			// SSL setup
			SSLContext ctx;
			try {
				ctx = SSLContext.getInstance("TLSv1.2");
				ctx.init(null, new TrustManager[] { tm }, new java.security.SecureRandom());
				huc.setSSLSocketFactory(ctx.getSocketFactory());

			} catch (NoSuchAlgorithmException e) {
				throw new RuntimeException(e);
			} catch (KeyManagementException e) {
				throw new RuntimeException(e);
			}

		}

	}

}
