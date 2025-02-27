/*
Copyright 2019 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/util/retry"
)

func fileExists(fname string) error {
	check := func() error {
		info, err := os.Stat(fname)
		if os.IsNotExist(err) {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("error expect file got dir")
		}
		return nil
	}

	if err := retry.Expo(check, 1*time.Second, 3); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed check if file (%q) exists,", fname))
	}
	return nil
}

// TestVersionUpgrade downloads latest version of minikube and runs with
// the odlest supported k8s version and then runs the current head minikube
// and it tries to upgrade from the older supported k8s to news supported k8s
func TestVersionUpgrade(t *testing.T) {
	p := profileName(t)
	if shouldRunInParallel(t) {
		t.Parallel()
	}
	// fname is the filename for the minikube's latetest binary. this file been pre-downloaded before test by hacks/jenkins/common.sh
	fname := filepath.Join(*testdataDir, fmt.Sprintf("minikube-%s-%s-latest-stable", runtime.GOOS, runtime.GOARCH))
	err := fileExists(fname)
	if err != nil { // download file if it is not downloaded by other test
		dest := filepath.Join(*testdataDir, fmt.Sprintf("minikube-%s-%s-latest-stable", runtime.GOOS, runtime.GOARCH))
		if runtime.GOOS == "windows" {
			dest += ".exe"
		}
		err := downloadMinikubeBinary(t, dest, "latest")
		if err != nil {
			// binary is needed for the test
			t.Fatalf("erorr downloading the latest minikube release %v", err)
		}
	}
	defer os.Remove(fname)

	mkCurrent := NewMinikubeRunner(t, p)
	defer mkCurrent.TearDown(t)

	mkRelease := NewMinikubeRunner(t, p)
	mkRelease.BinaryPath = fname
	// For full coverage: also test upgrading from oldest to newest supported k8s release
	stdout, stderr, err := mkRelease.Start(fmt.Sprintf("--kubernetes-version=%s", constants.OldestKubernetesVersion))
	if err != nil {
		t.Fatalf("TestVersionUpgrade minikube start failed : %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	mkRelease.CheckStatus(state.Running.String())
	mkRelease.RunCommand("stop", true)
	mkRelease.CheckStatus(state.Stopped.String())

	// Trim the leading "v" prefix to assert that we handle it properly.
	stdout, stderr, err = mkCurrent.Start(fmt.Sprintf("--kubernetes-version=%s", strings.TrimPrefix(constants.NewestKubernetesVersion, "v")))
	if err != nil {
		t.Fatalf("TestVersionUpgrade mkCurrent.Start start failed : %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	mkCurrent.CheckStatus(state.Running.String())
}
