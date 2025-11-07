//go:build linux || freebsd

package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v6/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = FDescribe("Podman load", func() {

	BeforeEach(func() {
		podmanTest.AddImageToRWStore(ALPINE)
	})

	FIt("podman load perf", func() {
		podmanImage := "quay.io/containers/podman@sha256:7dbeb8b7a8f83ef96bad4c4f04fbdecedc65d36a5c6953a1861eb94386f3c4ec"
		outTar := filepath.Join("/tmp", "podman.tar")
		outDir := filepath.Join("/tmp", "podman_dir")

		// Pull and save podman image first
		pull := podmanTest.Podman([]string{"pull", "-q", podmanImage})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(ExitCleanly())

		save := podmanTest.Podman([]string{"save", "-q", "-o", outTar, podmanImage})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		save = podmanTest.Podman([]string{"save", "-q", "--format", "oci-dir", "-o", outDir, podmanImage})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		// Reset
		rmi := podmanTest.Podman([]string{"system", "reset", "-f"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		// Create log file for performance data in working directory
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		logFile := filepath.Join(cwd, "load_perf.log")
		f, err := os.Create(logFile)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		// Create profile files in working directory
		// cpuProfile := filepath.Join(cwd, "load_cpu.prof")
		// memProfile := filepath.Join(cwd, "load_mem.prof")

		// Create a multi-writer that writes to both file and GinkgoWriter
		multiWriter := io.MultiWriter(f, GinkgoWriter)

		// Load with performance logging - pass --log-level=info and profiling flags
		// result := podmanTest.PodmanExecBaseWithOptions([]string{"--log-level=info", "--cpu-profile", cpuProfile, "--memory-profile", memProfile, "load", "-q", "-i", outfile}, PodmanExecOptions{
		result := podmanTest.PodmanExecBaseWithOptions([]string{"--log-level=info", "load", "-q", "-i", outTar}, PodmanExecOptions{
			FullOutputWriter: multiWriter,
		})
		result.WaitWithDefaultTimeout()
		// Expect(result).Should(ExitCleanly())
	})

	It("podman load input flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		GinkgoWriter.Println(images.OutputToStringArray())

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman load compressed tar file", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		compress := SystemExec("gzip", []string{outfile})
		Expect(compress).Should(ExitCleanly())
		outfile += ".gz"

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman load oci-archive image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman load oci-archive with signature", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, "--format", "oci-archive", ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "--signature-policy", "/etc/containers/policy.json", "-i", outfile})
		result.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(result).To(ExitWithError(125, "unknown flag: --signature-policy"))
		} else {
			Expect(result).Should(ExitCleanly())
		}
	})

	It("podman load with quiet flag", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman load directory", func() {
		SkipIfRemote("Remote does not support loading directories")
		outdir := filepath.Join(podmanTest.TempDir, "alpine")

		save := podmanTest.Podman([]string{"save", "-q", "--format", "oci-dir", "-o", outdir, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outdir})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman-remote load directory", func() {
		// Remote-only test looking for the specific remote error
		// message when trying to load a directory.
		if !IsRemote() {
			Skip("Remote only test")
		}

		result := podmanTest.Podman([]string{"load", "-i", podmanTest.TempDir})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, fmt.Sprintf("remote client supports archives only but %q is a directory", podmanTest.TempDir)))
	})

	It("podman load bogus file", func() {
		save := podmanTest.Podman([]string{"load", "-i", "foobar.tar"})
		save.WaitWithDefaultTimeout()
		Expect(save).To(ExitWithError(125, "faccessat foobar.tar: no such file or directory"))
	})

	It("podman load multiple tags", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("skip on ppc64le")
		}
		outfile := filepath.Join(podmanTest.TempDir, "alpine.tar")
		alpVersion := "quay.io/libpod/alpine:3.10.2"

		pull := podmanTest.Podman([]string{"pull", "-q", alpVersion})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(ExitCleanly())

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE, alpVersion})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE, alpVersion})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ALPINE})
		inspect.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(alpVersion))

		inspect = podmanTest.Podman([]string{"inspect", alpVersion})
		inspect.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(alpVersion))
	})

	It("podman load localhost registry from scratch", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")
		setup := podmanTest.Podman([]string{"tag", ALPINE, "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		setup = podmanTest.Podman([]string{"save", "-q", "-o", outfile, "--format", "oci-archive", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		setup = podmanTest.Podman([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		load := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"images", "hello:world"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(Not(ContainSubstring("docker")))
		Expect(result.OutputToString()).To(ContainSubstring("localhost"))
	})

	It("podman load localhost registry from scratch and :latest", func() {
		outfile := filepath.Join(podmanTest.TempDir, "load_test.tar.gz")

		setup := podmanTest.Podman([]string{"tag", ALPINE, "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		setup = podmanTest.Podman([]string{"save", "-q", "-o", outfile, "--format", "oci-archive", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		setup = podmanTest.Podman([]string{"rmi", "hello"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		load := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"images", "hello:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToString()).To(Not(ContainSubstring("docker")))
		Expect(result.OutputToString()).To(ContainSubstring("localhost"))
	})

	It("podman load localhost registry from dir", func() {
		SkipIfRemote("podman-remote does not support loading directories")
		outfile := filepath.Join(podmanTest.TempDir, "load")

		setup := podmanTest.Podman([]string{"tag", ALPINE, "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		setup = podmanTest.Podman([]string{"save", "-q", "-o", outfile, "--format", "oci-dir", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		setup = podmanTest.Podman([]string{"rmi", "hello:world"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		load := podmanTest.Podman([]string{"load", "-q", "-i", outfile})
		load.WaitWithDefaultTimeout()
		Expect(load).Should(ExitCleanly())
		Expect(load.OutputToString()).To(ContainSubstring("Loaded image: sha256:"))
	})

	It("podman load xz compressed image", func() {
		outfile := filepath.Join(podmanTest.TempDir, "alp.tar")

		save := podmanTest.Podman([]string{"save", "-q", "-o", outfile, ALPINE})
		save.WaitWithDefaultTimeout()
		Expect(save).Should(ExitCleanly())
		session := SystemExec("xz", []string{outfile})
		Expect(session).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", ALPINE})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"load", "-q", "-i", outfile + ".xz"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman load multi-image archive", func() {
		result := podmanTest.Podman([]string{"load", "-i", "./testdata/docker-two-images.tar.xz"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring("example.com/empty:latest"))
		Expect(result.OutputToString()).To(ContainSubstring("example.com/empty/but:different"))

		stderr := result.ErrorToString()
		if IsRemote() {
			Expect(stderr).To(BeEmpty(), "no stderr when running remote")
		} else {
			Expect(stderr).To(ContainSubstring("Getting image source signatures"))
			Expect(stderr).To(ContainSubstring("Copying blob"))
			Expect(stderr).To(ContainSubstring("Writing manifest to image destination"))
			Expect(stderr).To(ContainSubstring("Copying config sha256:"))
		}
	})
})
