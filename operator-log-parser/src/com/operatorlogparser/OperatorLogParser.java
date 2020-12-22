package com.operatorlogparser;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.text.SimpleDateFormat;
import java.util.Arrays;
import java.util.Date;
import java.util.Map;

import com.google.gson.Gson;

public class OperatorLogParser {

	private static final SimpleDateFormat sdf = new SimpleDateFormat("HH:mm:ss.SSS");

	public static void main(String[] args) throws IOException {

		Gson gson = new Gson();

		BufferedReader br = new BufferedReader(new InputStreamReader(System.in));

		String str;
		while (null != (str = br.readLine())) {
			processLine(str, gson);
		}

	}

	private static final String CRLF = System.getProperty("line.separator");

	private static void processLine(String e, Gson gson) {
		try {
			String result = "";

			@SuppressWarnings("unchecked")
			Map<String, Object> map = gson.fromJson(e, Map.class);

			Double ts = (Double) map.get("ts");
			if (ts != null) {
				Date d = new Date((long) (ts * 1000));
				result += "(" + sdf.format(d) + ") ";
			}

			String msg = (String) map.get("msg");
			result += msg + CRLF;

			String error = (String) map.get("error");
			if (error != null) {
				result += "- " + error + CRLF;
			}

			String stackTrace = (String) map.get("stacktrace");
			if (stackTrace != null) {

				for (String line : Arrays.asList(stackTrace.split("\\r?\\n"))) {
					result += "    " + line + CRLF;
				}
			}

			System.out.println(result);

		} catch (Exception ex) {
			System.out.println("! " + e);
			System.out.println();
		}

	}
}
