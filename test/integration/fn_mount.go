// +build integration

/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	pkgutil "k8s.io/minikube/pkg/util"
	"k8s.io/minikube/pkg/util/lock"
	"k8s.io/minikube/pkg/util/retry"
	"k8s.io/minikube/test/integration/util"
)

func testMounting(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("mount tests disabled in darwin due to timeout (issue#3200)")
	}
	if isTestNoneDriver(t) {
		t.Skip("skipping test for none driver as it does not need mount")
	}

	t.Parallel()
	p := profileName(t)
	mk := NewMinikubeRunner(t, p, "--wait=false")

	tempDir, err := ioutil.TempDir("", "mounttest")
	if err != nil {
		t.Fatalf("Unexpected error while creating tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mountCmd := getMountCmd(mk, tempDir)
	cmd, _, _ := mk.RunDaemon2(mountCmd)
	defer func() {
		err := cmd.Process.Kill()
		if err != nil {
			t.Logf("Failed to kill mount command: %v", err)
		}
	}()

	kr := util.NewKubectlRunner(t, p)
	podName := "busybox-mount"
	podPath := filepath.Join(*testdataDir, "busybox-mount-test.yaml")
	// Write file in mounted dir from host
	expected := "test\n"
	if err := writeFilesFromHost(tempDir, []string{"fromhost", "fromhostremove"}, expected); err != nil {
		t.Fatalf(err.Error())
	}

	// Create the pods we need outside the main test loop.
	setupTest := func() error {
		t.Logf("Deploying pod from: %s", podPath)
		if _, err := kr.RunCommand([]string{"create", "-f", podPath}); err != nil {
			return err
		}
		return nil
	}

	defer func() {
		t.Logf("Deleting pod from: %s", podPath)
		if out, err := kr.RunCommand([]string{"delete", "-f", podPath}); err != nil {
			t.Logf("delete -f %s failed: %v\noutput: %s\n", podPath, err, out)
		}
	}()

	if err = retry.Expo(setupTest, 500*time.Millisecond, 4*time.Minute); err != nil {
		t.Fatal("mountTest failed with error:", err)
	}

	if err := waitForPods(map[string]string{"integration-test": "busybox-mount"}, p); err != nil {
		t.Fatalf("Error waiting for busybox mount pod to be up: %v", err)
	}
	t.Logf("Pods appear to be running")

	mountTest := func() error {
		if err := verifyFiles(mk, kr, tempDir, podName, expected); err != nil {
			t.Fatalf(err.Error())
		}

		return nil
	}

	if err = retry.Expo(mountTest, 500*time.Millisecond, 4*time.Minute); err != nil {
		t.Fatalf("mountTest failed with error: %v", err)
	}

}

func getMountCmd(mk util.MinikubeRunner, mountDir string) string {
	var mountCmd string
	if len(mk.MountArgs) > 0 {
		mountCmd = fmt.Sprintf("mount %s %s:/mount-9p", mk.MountArgs, mountDir)
	} else {
		mountCmd = fmt.Sprintf("mount %s:/mount-9p", mountDir)
	}
	return mountCmd
}

func writeFilesFromHost(mountedDir string, files []string, content string) error {
	for _, file := range files {
		path := filepath.Join(mountedDir, file)
		err := lock.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("unexpected error while writing file %s: %v", path, err)
		}
	}
	return nil
}

func waitForPods(s map[string]string, profile string) error {
	client, err := pkgutil.GetClient(profile)
	if err != nil {
		return fmt.Errorf("getting kubernetes client: %v", err)
	}
	selector := labels.SelectorFromSet(labels.Set(s))
	if err := pkgutil.WaitForPodsWithLabelRunning(client, "default", selector); err != nil {
		return err
	}
	return nil
}

func verifyFiles(mk util.MinikubeRunner, kr *util.KubectlRunner, tempDir string, podName string, expected string) error {
	path := filepath.Join(tempDir, "frompod")
	out, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	// test that file written from pod can be read from host echo test > /mount-9p/frompod; in pod
	if string(out) != expected {
		return fmt.Errorf("expected file %s to contain text %q, was %q", path, expected, out)
	}

	// test that file written from host was read in by the pod via cat /mount-9p/fromhost;
	if out, err = kr.RunCommand([]string{"logs", podName}); err != nil {
		return err
	}
	if string(out) != expected {
		return fmt.Errorf("expected file %s to contain text %q, was %q", path, expected, out)
	}

	// test file timestamps are correct
	files := []string{"fromhost", "frompod"}
	for _, file := range files {
		statCmd := fmt.Sprintf("stat /mount-9p/%s", file)
		statOutput, err := mk.SSH(statCmd)
		if err != nil {
			return fmt.Errorf("inable to stat %s via SSH. error %v, %s", file, err, statOutput)
		}

		if runtime.GOOS == "windows" {
			if strings.Contains(statOutput, "Access: 1970-01-01") {
				return fmt.Errorf("invalid access time\n%s", statOutput)
			}
		}

		if strings.Contains(statOutput, "Modify: 1970-01-01") {
			return fmt.Errorf("invalid modify time\n%s", statOutput)
		}
	}

	// test that fromhostremove was deleted by the pod from the mount via rm /mount-9p/fromhostremove
	path = filepath.Join(tempDir, "fromhostremove")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("expected file %s to be removed", path)
	}

	// test that frompodremove can be deleted on the host
	path = filepath.Join(tempDir, "frompodremove")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("unexpected error removing file %s: %v", path, err)
	}

	return nil
}
