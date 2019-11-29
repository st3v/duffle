package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/deislabs/duffle/pkg/duffle/home"
)

func TestBundleRemove(t *testing.T) {
	tempDuffleHome := mustSetupTempDuffleHome(t)
	defer os.Remove(tempDuffleHome)
	mustCopyTestBundle(t, tempDuffleHome)

	cmd := bundleRemoveCmd{
		home:      home.Home(tempDuffleHome),
		bundleRef: "foo",
		out:       ioutil.Discard,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("Did not expect error, got %s", err)
	}

	if _, err := os.Stat(filepath.Join(cmd.home.Bundles(), "foo-1.0.0.json")); !os.IsNotExist(err) {
		t.Errorf("Expected bundle file to be removed from local store but was not")
	}
}
