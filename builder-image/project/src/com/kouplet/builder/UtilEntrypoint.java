package com.kouplet.builder;

import java.io.IOException;
import java.io.InputStream;
import java.net.URL;
import java.nio.file.FileVisitResult;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.StandardCopyOption;
import java.nio.file.attribute.BasicFileAttributes;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

public class UtilEntrypoint {

	private static final String URL_PREFIX = "kouplet-url-";

	private static final String ENV_GIT_REPO = "kouplet-image-git-repository-url";

	private static final String IMAGE = "kouplet-image";

	private static final String REGISTRY_USERNAME = "kouplet-registry-username";
	private static final String REGISTRY_PASSWORD = "kouplet-registry-password";
	private static final String REGISTRY_HOST = "kouplet-registry-host";

	private static final String GIT_REPO_SUBPATH = "kouplet-repository-subpath";

	private static Path repoResourcesPath = null;

	public static void main(String[] args) throws IOException, InterruptedException {

		Map<String, String> env = System.getenv();

		List<String> urls = env.entrySet().stream().filter(e -> e.getKey().startsWith(URL_PREFIX))
				.map(e -> e.getValue()).collect(Collectors.toList());

		if (urls.size() == 0) {
			System.err.println("* No URLs were specified in the CR YAML.");
		}

		String gitRepositoryUrl = env.get(ENV_GIT_REPO);

		if (gitRepositoryUrl == null) {
			System.err.println("Git registry URL not found: " + env);
			fail();
		}

		Path sshKey = Paths.get("/etc/git-ssh-key-secret-volume/ssh-privatekey");

		if (!Files.exists(sshKey)) {
			sshKey = Paths.get("/ssh-credentials/id_rsa");
		}

		if (Files.exists(sshKey)) {
			System.out.println();
			System.out.println("* Creating ~/.ssh and copying id_rsa file");
			Path userHome = Paths.get(System.getProperty("user.home"), ".ssh");
			Files.createDirectories(userHome);

			Path destFile = userHome.resolve("id_rsa");

			Files.copy(sshKey, destFile, StandardCopyOption.REPLACE_EXISTING);
			Files.setPosixFilePermissions(destFile, PosixFilePermissions.fromString("rw-------"));
		} else {
			System.out.println("* No SSH credentials specified.");
		}

		Path workingDir;
		{
			workingDir = Paths.get(System.getProperty("user.home"));

			if (!Files.exists(workingDir)) {
				throw new RuntimeException("Git clone target dir doesn't exist: " + workingDir);
			}

			System.out.println();
			System.out.println("* Cloning git repository to " + workingDir);

			ProcessBuilder pb = new ProcessBuilder("git", "clone", gitRepositoryUrl);
			Map<String, String> procEnv = pb.environment();
			procEnv.put("GIT_SSH_COMMAND", "ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no");

			Process p = pb.inheritIO().directory(workingDir.toFile()).start();
			p.waitFor();
			assertSuccess(p);
		}

		String repoName = gitRepositoryUrl.substring(gitRepositoryUrl.lastIndexOf("/") + 1);

		Path repoRoot = workingDir.resolve(repoName);
		{
			if (!Files.exists(repoRoot)) {
				System.err.println("Repo root doesn't exist: " + repoRoot);
				fail();
			}

			String gitRepoSubpath = env.get(GIT_REPO_SUBPATH);
			if (gitRepoSubpath != null) {
				gitRepoSubpath = gitRepoSubpath.trim();
				while (gitRepoSubpath.startsWith("/")) {
					gitRepoSubpath = gitRepoSubpath.substring(1);
				}
				while (gitRepoSubpath.endsWith("/")) {
					gitRepoSubpath = gitRepoSubpath.substring(0, gitRepoSubpath.length() - 1);
				}
			}

			if (gitRepoSubpath != null && gitRepoSubpath.length() > 0) {
				repoRoot = repoRoot.resolve(gitRepoSubpath);
			}

			if (!Files.exists(repoRoot)) {
				System.err.println("Modified repo root doesn't exist: '" + repoRoot + "', sub-path value is: '"
						+ gitRepoSubpath + "'");
				fail();
			}

		}

		System.out.println();
		Path repoResources = repoRoot.resolve("resources");
		repoResourcesPath = repoResources;
		Files.createDirectories(repoResources);

		for (String url : urls) {

			System.out.println("* Downloading " + url);

			String filename = url.substring(url.lastIndexOf("/") + 1);

			InputStream is = HttpUtil.get(new URL(url));

			Files.copy(is, repoResources.resolve(filename));

		}

		// Buildah login
		{
			System.out.println();
			System.out.println("* Logging into container registry");

			String registryUsername = env.get(REGISTRY_USERNAME);
			if (registryUsername == null) {
				System.err.println("Registry user name is not specified.");
				return;
			}

			String registryPassword = env.get(REGISTRY_PASSWORD);
			if (registryPassword == null) {
				System.err.println("Registry password is not specified.");
				return;
			}

			String registryHost = env.get(REGISTRY_HOST);
			if (registryHost == null) {
				System.err.println("Registry password is not specified.");
				return;
			}

			List<String> items = Arrays.asList("buildah", "login", "-u", registryUsername, "-p", registryPassword,
					registryHost);
			ProcessBuilder pb = new ProcessBuilder(items);
			Process p = pb.inheritIO().directory(repoRoot.toFile()).start();
			p.waitFor();
			assertSuccess(p);
		}

		String image = env.get(IMAGE);
		if (image == null) {
			System.err.println("Image name is not specified.");
			fail();
		}

		// Buildah bud
		{
			System.out.println();
			System.out.println("Building image");

			ProcessBuilder pb = new ProcessBuilder("buildah", "bud", "--squash", "--no-cache", "--disable-compression",
					"-t", image);
			Process p = pb.inheritIO().directory(repoRoot.toFile()).start();
			p.waitFor();
			assertSuccess(p);
		}

		// Push tag
		{

			System.out.println("Deleting boltdb");

			try {
				deleteDirectory(Paths.get("/var/lib/containers/cache/blob-info-cache-v1.boltdb"));
			} catch (Throwable t) {
				/* ignore */
			}

			try {
				Files.delete(Paths.get("/var/lib/containers/cache/blob-info-cache-v1.boltdb"));
			} catch (Throwable t) {
				/* ignore */
			}

			System.out.println();
			System.out.println("Push image");

			ProcessBuilder pb = new ProcessBuilder("buildah", "push", "--disable-compression", image);
			Process p = pb.inheritIO().directory(repoRoot.toFile()).start();
			p.waitFor();
			assertSuccess(p);
		}

//		int val = (int) (Math.random() * 1000d);
//
//		if (val < 250) {
//			System.out.println("SUCCESS " + val);
//			System.exit(0);
//		} else {
//			System.out.println("FAIL " + val);
//			System.exit(1);
//		}

		System.out.println("SUCCESS");
		disposeOfResources(repoResourcesPath);
		System.exit(0);
	}

	private static void assertSuccess(Process p) {
		if (p.exitValue() != 0) {
			System.err.println("FAIL: Process exitted with non-zero status code.");
			disposeOfResources(repoResourcesPath);
			System.exit(1);
		}
	}

	private static void fail() {
		System.exit(1);
	}

	private static void disposeOfResources(Path repoResources) {
		try {
			List<String> toDelete = new ArrayList<>(Arrays.asList("/root/.m2", "/var/lib/containers"));

			if (repoResources != null) {
				toDelete.add(repoResources.toString());
			}

			toDelete.stream().map(e -> Paths.get(e)).forEach(e -> {
				try {
					deleteDirectory(e);
				} catch (IOException e1) {
					e1.printStackTrace();
				}
			});

		} catch (Throwable t) {
			// Print then ignore
			t.printStackTrace();
		}
	}

	private static void deleteDirectory(Path pathToDelete) throws IOException {
		Files.walkFileTree(pathToDelete, new SimpleFileVisitor<Path>() {
			@Override
			public FileVisitResult visitFile(Path file, BasicFileAttributes attrs) throws IOException {
				try {
					Files.delete(file);
				} catch (IOException e) {
					System.err.println(file + " ->" + e.getClass().getName() + " " + e.getMessage());
				}
				return FileVisitResult.CONTINUE;
			}

			@Override
			public FileVisitResult postVisitDirectory(Path dir, IOException exc) throws IOException {
				try {
					Files.delete(dir);
				} catch (IOException e) {
					System.err.println(dir + " ->" + e.getClass().getName() + " " + e.getMessage());
				}
				return FileVisitResult.CONTINUE;
			}
		});
	}

}
