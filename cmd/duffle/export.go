package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/deislabs/cnab-go/bundle/loader"
	"github.com/pivotal/image-relocation/pkg/transport"
	"github.com/spf13/cobra"

	"github.com/deislabs/duffle/pkg/duffle/home"
	"github.com/deislabs/duffle/pkg/imagestore"
	"github.com/deislabs/duffle/pkg/imagestore/construction"
	"github.com/deislabs/duffle/pkg/packager"
)

const exportDesc = `
Packages a bundle, and by default any images referenced by the bundle, within a single gzipped tarfile.

Unless --thin is specified, a thick bundle is exported. A thick bundle contains the bundle manifest and all images
(including invocation images) referenced by the bundle metadata. Images are saved as an OCI image layout in the
artifacts/layout/ directory.

If --thin specified, only the bundle manifest is exported.

By default, this command will use the name and version information of the bundle to create a compressed archive file
called <name>-<version>.tgz in the current directory. This destination can be updated by specifying a file path to save
the compressed bundle to using the --output-file flag.

A path to a bundle file may be passed in instead of a bundle in local storage by using the --bundle-is-file flag, thus:
$ duffle export [PATH] --bundle-is-file
`

type exportCmd struct {
	// args
	bundle string

	// flags
	dest          string
	thin          bool
	verbose       bool
	bundleIsFile  bool
	skipTLSVerify bool
	caCertPaths   []string

	// context
	home home.Home
	out  io.Writer

	// dependencies
	transportProvider             func([]string, bool) (*http.Transport, error)
	imageStoreConstructorProvider func(bool) imagestore.Constructor
}

func newExportCmd(w io.Writer) *cobra.Command {
	export := &exportCmd{
		out:                           w,
		transportProvider:             transport.NewHttpTransport,
		imageStoreConstructorProvider: construction.NewConstructor,
	}

	cmd := &cobra.Command{
		Use:   "export [BUNDLE]",
		Short: "package CNAB bundle in gzipped tar file",
		Long:  exportDesc,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			export.home = home.Home(homePath())
			export.bundle = args[0]

			return export.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&export.dest, "output-file", "o", "", "Save exported bundle to file path")
	f.BoolVarP(&export.bundleIsFile, "bundle-is-file", "f", false, "Interpret the bundle source as a file path")
	f.BoolVarP(&export.thin, "thin", "t", false, "Export only the bundle manifest")
	f.BoolVarP(&export.verbose, "verbose", "v", false, "Verbose output")
	f.StringSliceVarP(&export.caCertPaths, "ca-cert-path", "", nil, "Path to CA certificate for verifying registry TLS certificates (can be repeated for multiple certificates)")
	f.BoolVarP(&export.skipTLSVerify, "skip-tls-verify", "", false, "Skip TLS certificate verification for registries")

	return cmd
}

func (ex *exportCmd) run() error {
	bundlefile, l, err := ex.setup()
	if err != nil {
		return err
	}

	if err := ex.Export(bundlefile, l); err != nil {
		return err
	}

	return nil
}

func (ex *exportCmd) Export(bundlefile string, l loader.BundleLoader) error {
	ctor := func(opts ...imagestore.Option) (imagestore.Store, error) {
		transport, err := ex.transportProvider(ex.caCertPaths, ex.skipTLSVerify)
		if err != nil {
			return nil, err
		}

		opts = append(opts, imagestore.WithTransport(transport))
		return ex.imageStoreConstructorProvider(ex.thin)(opts...)
	}

	exp, err := packager.NewExporter(bundlefile, ex.dest, ex.home.Logs(), l, ctor)
	if err != nil {
		return fmt.Errorf("Unable to set up exporter: %s", err)
	}

	if err := exp.Export(); err != nil {
		return err
	}

	if ex.verbose {
		fmt.Fprintf(ex.out, "Export logs: %s\n", exp.Logs())
	}

	return nil
}

func (ex *exportCmd) setup() (string, loader.BundleLoader, error) {
	l := loader.New()

	bundlefile, err := resolveBundleFilePath(ex.bundle, ex.home.String(), ex.bundleIsFile)
	if err != nil {
		return "", l, err
	}

	return bundlefile, l, nil
}

func resolveBundleFilePath(bun, homePath string, bundleIsFile bool) (string, error) {
	if bundleIsFile {
		return bun, nil
	}

	bundlefile, err := getBundleFilepath(bun, homePath)
	if err != nil {
		return "", err
	}

	return bundlefile, err
}
