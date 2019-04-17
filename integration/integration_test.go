package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/dagger"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	spec.Run(t, "Integration", testIntegration, spec.Report(report.Terminal{}))
}

func testIntegration(t *testing.T, when spec.G, it spec.S) {
	var (
		pythonURI string
		pipURI    string
		err       error
	)

	it.Before(func() {
		RegisterTestingT(t)
		pipURI, err = dagger.PackageBuildpack()
		Expect(err).ToNot(HaveOccurred())
		pythonURI, err = dagger.GetLatestBuildpack("python-cnb")
		Expect(err).ToNot(HaveOccurred())
	})

	it.After(func() {
		if pipURI != "" {
			Expect(os.RemoveAll(pipURI)).To(Succeed())
		}
	})

	when("building a simple app", func() {
		it("runs a python app using pip", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "simple_app"), pythonURI, pipURI)
			Expect(err).ToNot(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			app.Env["PORT"] = "8080"

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				containerID, imageName, volumeIDs, err := app.Info()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

				containerLogs, err := app.Logs()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("Container Logs:\n %s\n", containerLogs)
				t.FailNow()
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))

			Expect(app.Destroy()).To(Succeed())
		})

		it("caches reused modules for the same app, but downloads new modules ", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "simple_app"), pythonURI, pipURI)
			Expect(err).ToNot(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			app.Env["PORT"] = "8080"
			err = app.Start()
			Expect(err).ToNot(HaveOccurred())

			_, imgName, _, _ := app.Info()
			rebuiltApp, err := dagger.PackBuildNamedImage(imgName, filepath.Join("testdata", "simple_app_more_packages"), pythonURI, pipURI)
			Expect(err).NotTo(HaveOccurred())
			Expect(rebuiltApp.BuildLogs()).To(MatchRegexp("Using cached.*Flask"))
			Expect(rebuiltApp.BuildLogs()).To(MatchRegexp("Downloading.*itsdangerous"))
			Expect(rebuiltApp.Destroy()).To(Succeed())
		})

		it("does not reinstall python_packages and pip cache", func() {
			pipEnvURI, err := dagger.PackageLocalBuildpack("pipenv-cnb", "/Users/pivotal/workspace/pipenv-cnb")
			Expect(err).NotTo(HaveOccurred())

			appDir := filepath.Join("testdata", "with_pipfile_lock")
			app, err := dagger.PackBuild(appDir, pythonURI, pipEnvURI, pipURI)
			Expect(err).ToNot(HaveOccurred())

			logs := app.BuildLogs()
			Expect(logs).To(MatchRegexp("Python Packages (\\S)+: Contributing to layer"))
			Expect(logs).To(MatchRegexp("PIP Cache (\\S)+: Contributing to layer"))

			_, imageID, _, err := app.Info()
			Expect(err).NotTo(HaveOccurred())
			rebuiltApp, err := dagger.PackBuildNamedImage(imageID, filepath.Join("testdata", "with_pipfile_lock"), pythonURI, pipEnvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			logs = rebuiltApp.BuildLogs()
			Expect(logs).To(MatchRegexp("Python Packages (\\S)+: Reusing cached layer"))
			Expect(logs).NotTo(MatchRegexp("Python Packages (\\S)+: Contributing to layer"))
			Expect(logs).To(MatchRegexp("PIP Cache (\\S)+: Reusing cached layer"))
			Expect(logs).NotTo(MatchRegexp("PIP Cache (\\S)+: Contributing to layer"))

			rebuiltApp.SetHealthCheck("", "3s", "1s")
			rebuiltApp.Env["PORT"] = "8080"
			Expect(rebuiltApp.Start()).To(Succeed())
			_, _, err = rebuiltApp.HTTPGet("/")
			Expect(err).NotTo(HaveOccurred())
			Expect(rebuiltApp.Destroy()).To(Succeed())
		})
	})

	when("building a simple app that is vendored", func() {
		it("runs a python app using pip", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "simple_app"), pythonURI, pipURI)
			Expect(err).ToNot(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			app.Env["PORT"] = "8080"

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				containerID, imageName, volumeIDs, err := app.Info()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

				containerLogs, err := app.Logs()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("Container Logs:\n %s\n", containerLogs)
				t.FailNow()
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World!"))

			Expect(app.Destroy()).To(Succeed())
		})
	})
}
