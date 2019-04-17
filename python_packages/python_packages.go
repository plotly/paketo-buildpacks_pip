package python_packages

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/fatih/color"

	"github.com/buildpack/libbuildpack/application"
	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
)

const (
	Dependency       = "python_packages"
	Cache            = "pip_cache"
	Cacheable        = "cacheable"
	CacheLayer       = "python_packages_cache"
	RequirementsFile = "requirements.txt"
)

type PackageManager interface {
	Install(requirementsPath, location, cacheDir string) error
	InstallVendor(requirementsPath, location, vendorDir string) error
}

type MetadataInterface interface {
	Identity() (name string, version string)
}

type Metadata struct {
	Name string
	Hash string
}

func (m Metadata) Identity() (name string, version string) {
	return m.Name, m.Hash
}

type Contributor struct {
	PythonPackagesMetadata MetadataInterface
	PipCacheMetadata       MetadataInterface
	manager                PackageManager
	app                    application.Application
	packagesLayer          layers.Layer
	launchLayer            layers.Layers
	cacheLayer             layers.Layer
	buildContribution      bool
	launchContribution     bool
}

func NewContributor(context build.Build, manager PackageManager) (Contributor, bool, error) {
	dep, willContribute := context.BuildPlan[Dependency]
	if !willContribute {
		return Contributor{}, false, nil
	}

	requirementsFile := filepath.Join(context.Application.Root, "requirements.txt")
	if exists, err := helper.FileExists(requirementsFile); err != nil {
		return Contributor{}, false, err
	} else if !exists {
		return Contributor{}, false, fmt.Errorf(`unable to find "requirements.txt"`)
	}

	contributor := Contributor{
		manager:       manager,
		app:           context.Application,
		packagesLayer: context.Layers.Layer(Dependency),
		cacheLayer:    context.Layers.Layer(Cache),
		launchLayer:   context.Layers,
	}

	cacheDep, contributeToLayerCache := context.BuildPlan[CacheLayer]
	if contributeToLayerCache {
		if hashString, ok := cacheDep.Metadata[Cacheable].(string); ok {
			context.Logger.Warning("Hash String: %s", hashString)
			contributor.PipCacheMetadata = Metadata{Cache, hashString}
			contributor.PythonPackagesMetadata = Metadata{Dependency, hashString}
		}
	}

	if _, ok := dep.Metadata["build"]; ok {
		contributor.buildContribution = true
	}

	if _, ok := dep.Metadata["launch"]; ok {
		contributor.launchContribution = true
	}

	return contributor, true, nil
}

func (c Contributor) Contribute() error {
	if err := c.contributePythonModules(); err != nil {
		return err
	}

	if err := c.contributePipCache(); err != nil {
		return err
	}

	return c.contributeStartCommand()
}

// We never check if layer metadata matches :(
// as a result we always contribute
// probably want to re-write this and use the contribute method...

//func (c Contributor) contributePythonModuelesNEW() error {
//	return c.packagesLayer.Contribute(c.PythonPackagesMetadata, func(pythonLayer layers.Layer) error {
//		requirements := filepath.Join(c.app.Root, RequirementsFile)
//		vendorDir := filepath.Join(c.app.Root, "vendor")
//
//		vendored, err := helper.FileExists(vendorDir)
//		if err != nil {
//			return fmt.Errorf("unable to stat vendor dir: %s", err.Error())
//		}
//
//		if vendored {
//			c.packagesLayer.Logger.Info("pip installing from vendor directory")
//			if err := c.manager.InstallVendor(requirements, c.packagesLayer.Root, vendorDir); err != nil {
//				return err
//			}
//		} else {
//			c.packagesLayer.Logger.Info("pip installing to: " + c.packagesLayer.Root)
//			if err := c.manager.Install(requirements, c.packagesLayer.Root, c.cacheLayer.Root); err != nil {
//				return err
//			}
//		}
//
//		if err := c.packagesLayer.AppendPathSharedEnv("PYTHONUSERBASE", c.packagesLayer.Root); err != nil {
//			return err
//		}
//
//		if c.PythonPackagesMetadata != nil {
//			name, hash := c.PythonPackagesMetadata.Identity()
//			c.packagesLayer.Logger.Error("Identity function!!!!!!!!!!!!!!!!!!!!!! %s %s", name, hash)
//		}
//		return err
//	}, c.flags()...)
//}

func (c Contributor) contributePythonModules() error {
	c.packagesLayer.Touch()

	matches, err := c.packagesLayer.MetadataMatches(c.PythonPackagesMetadata)
	if err != nil {
		return err
	}

	if matches {
		c.packagesLayer.Logger.FirstLine("%s: %s cached layer",
			c.packagesLayer.Logger.PrettyIdentity(pythonPackagesID{}), color.GreenString("Reusing"))
		return c.packagesLayer.WriteMetadata(c.PythonPackagesMetadata, c.flags()...)
	}

	c.packagesLayer.Logger.FirstLine("%s: %s to layer",
		c.packagesLayer.Logger.PrettyIdentity(pythonPackagesID{}), color.YellowString("Contributing"))

	requirements := filepath.Join(c.app.Root, RequirementsFile)
	vendorDir := filepath.Join(c.app.Root, "vendor")

	vendored, err := helper.FileExists(vendorDir)
	if err != nil {
		return fmt.Errorf("unable to stat vendor dir: %s", err.Error())
	}

	if vendored {
		c.packagesLayer.Logger.Info("pip installing from vendor directory")
		if err := c.manager.InstallVendor(requirements, c.packagesLayer.Root, vendorDir); err != nil {
			return err
		}
	} else {
		c.packagesLayer.Logger.Info("pip installing to: " + c.packagesLayer.Root)
		if err := c.manager.Install(requirements, c.packagesLayer.Root, c.cacheLayer.Root); err != nil {
			return err
		}
	}

	if err := c.packagesLayer.AppendPathSharedEnv("PYTHONUSERBASE", c.packagesLayer.Root); err != nil {
		return err
	}

	if c.PythonPackagesMetadata != nil {
		name, hash := c.PythonPackagesMetadata.Identity()
		c.packagesLayer.Logger.Error("Identity function!!!!!!!!!!!!!!!!!!!!!! %s %s", name, hash)

	}
	return c.packagesLayer.WriteMetadata(c.PythonPackagesMetadata, c.flags()...)
}

func (c Contributor) contributeStartCommand() error {
	procfile := filepath.Join(c.app.Root, "Procfile")
	exists, err := helper.FileExists(procfile)
	if err != nil {
		return err
	}

	if exists {
		buf, err := ioutil.ReadFile(procfile)
		if err != nil {
			return err
		}

		proc := regexp.MustCompile(`^\s*web\s*:\s*`).ReplaceAllString(string(buf), "")
		return c.launchLayer.WriteApplicationMetadata(layers.Metadata{Processes: []layers.Process{{"web", proc}}})
	}

	return nil
}

func (c Contributor) contributePipCache() error {
	if cacheExists, err := helper.FileExists(c.cacheLayer.Root); err != nil {
		return err
	} else if cacheExists {
		c.cacheLayer.Touch()

		matches, err := c.cacheLayer.MetadataMatches(c.PipCacheMetadata)
		if err != nil {
			return err
		}

		if matches {
			c.cacheLayer.Logger.FirstLine("%s: %s cached layer",
				c.cacheLayer.Logger.PrettyIdentity(pipCacheID{}), color.GreenString("Reusing"))
			return c.cacheLayer.WriteMetadata(c.PipCacheMetadata, c.flags()...)
		}

		c.cacheLayer.Logger.FirstLine("%s: %s to layer",
			c.cacheLayer.Logger.PrettyIdentity(pipCacheID{}), color.YellowString("Contributing"))

		if c.PipCacheMetadata != nil {
			name, hash := c.PipCacheMetadata.Identity()
			c.packagesLayer.Logger.Error("Identity function!!!!!!!!!!!!!!!!!!!!!! %s %s", name, hash)

		}

		return c.cacheLayer.WriteMetadata(c.PipCacheMetadata, layers.Cache)
	}
	return nil
}

func (c Contributor) flags() []layers.Flag {
	flags := []layers.Flag{layers.Cache}

	if c.buildContribution {
		flags = append(flags, layers.Build)
	}

	if c.launchContribution {
		flags = append(flags, layers.Launch)
	}

	return flags
}

type pythonPackagesID struct {
}

func (p pythonPackagesID) Identity() (name string, description string) {
	return "Python Packages", "latest"
}

type pipCacheID struct {
}

func (p pipCacheID) Identity() (name string, description string) {
	return "PIP Cache", "latest"
}
