package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/deislabs/duffle/pkg/duffle/home"
	"github.com/deislabs/duffle/pkg/imagestore"
	"github.com/deislabs/duffle/pkg/imagestore/imagestoremocks"
	"github.com/stretchr/testify/assert"
)

func TestExportSetup(t *testing.T) {
	out := ioutil.Discard

	tempDuffleHome := mustSetupTempDuffleHome(t)
	defer os.Remove(tempDuffleHome)
	mustCopyTestBundle(t, tempDuffleHome)

	// Setup a temporary dir for destination
	tempDir := mustCreateTempDir(t, "duffledest")
	defer os.Remove(tempDir)

	duffleHome := home.Home(tempDuffleHome)
	exp := &exportCmd{
		bundle: "foo:1.0.0",
		dest:   tempDir,
		home:   duffleHome,
		out:    out,
	}

	source, _, err := exp.setup()
	if err != nil {
		t.Errorf("Did not expect error but got %s", err)
	}

	expectedSource := filepath.Join(tempDuffleHome, "bundles", "foo-1.0.0.json")
	if source != expectedSource {
		t.Errorf("Expected source to be %s, got %s", expectedSource, source)
	}

	expFail := &exportCmd{
		bundle: "bar:1.0.0",
		dest:   tempDir,
		home:   duffleHome,
		out:    out,
	}
	_, _, err = expFail.setup()
	if err == nil {
		t.Error("Expected error, got none")
	}

	bundlepath := filepath.Join("..", "..", "tests", "testdata", "bundles", "foo.json")
	expFile := &exportCmd{
		bundle:       bundlepath,
		dest:         tempDir,
		home:         duffleHome,
		out:          out,
		bundleIsFile: true,
	}
	source, _, err = expFile.setup()
	if err != nil {
		t.Errorf("Did not expect error but got %s", err)
	}

	if source != bundlepath {
		t.Errorf("Expected bundle file path to be %s, got %s", bundlepath, source)
	}
}

func TestExportTransport(t *testing.T) {
	tests := map[string]struct {
		expectedCertPaths     []string
		expectedSkipTLSVerify bool
		expectedErr           error
	}{
		"defaults": {
			nil,
			false,
			nil,
		},
		"one cert": {
			[]string{"a"},
			false,
			nil,
		},
		"multiple certs": {
			[]string{"a", "b", "c"},
			false,
			nil,
		},
		"skip TLS verify": {
			nil,
			true,
			nil,
		},
		"construction error": {
			nil,
			false,
			errors.New("i like turtles"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			homedir := mustSetupTempDuffleHome(t)
			defer os.RemoveAll(homedir)

			cmd := &exportCmd{
				bundle:        filepath.Join("..", "..", "tests", "testdata", "bundles", "foo.json"),
				out:           os.Stdout,
				home:          home.Home(homedir),
				dest:          filepath.Join(homedir, "test.tgz"),
				bundleIsFile:  true,
				skipTLSVerify: tc.expectedSkipTLSVerify,
				caCertPaths:   tc.expectedCertPaths,
			}

			expectedTransport := &http.Transport{}

			imageStoreConstructorProviderCalled := false
			cmd.imageStoreConstructorProvider = func(bool) imagestore.Constructor {
				return func(opts ...imagestore.Option) (imagestore.Store, error) {
					imageStoreConstructorProviderCalled = true

					p := imagestore.Parameters{}
					for _, opt := range opts {
						p = opt(p)
					}

					assert.Same(t, expectedTransport, p.Transport)

					return &imagestoremocks.MockStore{
						AddStub: func(string) (string, error) {
							return "added", nil
						},
					}, nil
				}
			}

			transportProviderCalled := false
			cmd.transportProvider = func(certPaths []string, skipTLSVerify bool) (*http.Transport, error) {
				transportProviderCalled = true
				assert.ElementsMatch(t, tc.expectedCertPaths, certPaths)
				assert.Equal(t, tc.expectedSkipTLSVerify, skipTLSVerify)
				return expectedTransport, tc.expectedErr
			}

			err := cmd.run()
			if tc.expectedErr != nil {
				assert.Error(t, err)
				// assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.Nil(t, err)
			}

			assert.True(t, transportProviderCalled)
			assert.Equal(t, tc.expectedErr == nil, imageStoreConstructorProviderCalled)
		})
	}
}

func mustSetupTempDuffleHome(t *testing.T) string {
	tempDuffleHome := mustCreateTempDir(t, "dufflehome")

	duffleHome := home.Home(tempDuffleHome)

	if err := os.MkdirAll(duffleHome.Bundles(), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(duffleHome.Logs(), 0755); err != nil {
		t.Fatal(err)
	}

	return tempDuffleHome
}

func mustCopyFile(t *testing.T, src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func mustCopyTestBundle(t *testing.T, tempDuffleHome string) {
	bun := filepath.Join("..", "..", "tests", "testdata", "bundles", "foo.json")
	outfile := "foo-1.0.0.json"
	mustCopyFile(t, bun, filepath.Join(tempDuffleHome, "bundles", outfile))

	jsonBlob := []byte(`{"foo": {"1.0.0": "foo-1.0.0.json"}}`)
	if err := ioutil.WriteFile(filepath.Join(tempDuffleHome, "repositories.json"), jsonBlob, 0644); err != nil {
		t.Fatal(err)
	}
}
