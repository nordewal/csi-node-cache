// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raid

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"k8s.io/klog/v2"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/util"
)

const (
	mdadmCmd   = "/bin/mdadm"
	mdstatFile = "/proc/mdstat"
)

var (
	mdstatInactive = regexp.MustCompile(`^([^ ]+) : inactive ([a-zA-Z0-9]+)`)
)

type RaidArray interface {
	Init() error
	Device() string
	Stop() error
}

type mirrorArray struct {
	target   string
	primary  string
	replicas []string
}

var _ RaidArray = &mirrorArray{}

type stripedArray struct {
	target  string
	devices []string
}

func NewMirrorArray(target, primary string, replicas ...string) RaidArray {
	return &mirrorArray{target: target, primary: primary, replicas: replicas}
}

func (m *mirrorArray) Device() string {
	return m.target
}

func (m *mirrorArray) Init() error {
	if err := validateDevice(m.primary); err != nil {
		return err
	}
	for _, dev := range m.replicas {
		if err := validateDevice(dev); err != nil {
			return err
		}
	}

	if err := stopAllInactive(); err != nil {
		return err
	}

	primaryIsRaid, err := isExistingRaidVolume(m.target, m.primary)
	if err != nil {
		return fmt.Errorf("Error when checking if %s is already a raid disk: %w", m.primary, err)
	}
	if primaryIsRaid {
		return assembleExistingMirror(m.target, m.primary, m.replicas...)
	}
	for _, repl := range m.replicas {
		replIsRaid, err := isExistingRaidVolume(m.target, repl)
		if err != nil {
			return fmt.Errorf("Error when checking if replica %s is aleady a raid disk: %s", repl, err)
		}
		if replIsRaid {
			return assembleExistingMirror(m.target, repl, slices.Concat([]string{m.primary}, m.replicas)...)
		}
	}
	return createNewMirror(m.target, slices.Concat([]string{m.primary}, m.replicas)...)
}

func (m *mirrorArray) Stop() error {
	return stopRaidDevice(m.Device())
}

func NewStripedArray(target string, devices ...string) RaidArray {
	return &stripedArray{target: target, devices: devices}
}

func (s *stripedArray) Device() string {
	return s.target
}

func (s *stripedArray) Init() error {
	if err := isRaidDevice(s.target); err == nil {
		return nil
	}

	for _, dev := range s.devices {
		if err := validateDevice(dev); err != nil {
			return err
		}
	}

	if err := stopAllInactive(); err != nil {
		return err
	}

	for _, dev := range s.devices {
		isRaid, err := isExistingRaidVolume(s.target, dev)
		if err != nil {
			return fmt.Errorf("Error when checking if devicce %s is already a raid disk: %s", dev, err)
		}
		if isRaid {
			return assembleExistingStriped(s.target, s.devices...)
		}
	}
	return createNewStriped(s.target, s.devices...)
}

func (s *stripedArray) Stop() error {
	return stopRaidDevice(s.Device())
}

func createNewMirror(target string, devices ...string) error {
	output, err := runMdadm(slices.Concat([]string{"--create", target, "--level", "1", "--run", "--raid-devices", fmt.Sprintf("%d", len(devices))}, devices)...)
	if err != nil {
		return fmt.Errorf("Mirror raid creation for %s={%v} failed (%w): %s", target, devices, err, output)
	}
	return nil
}

func assembleExistingMirror(target, existing string, devices ...string) error {
	for _, d := range devices {
		if d != existing {
			_ = wipeDevice(d) // Ignore any error, if there's a problem it will fail in the assemble
		}
	}
	output, err := runMdadm("--assemble", target, existing, "--run")
	if err != nil {
		return fmt.Errorf("Could not bootstrap assemble from %s (%w): %s", existing, err, output)
	}
	output, err = runMdadm(slices.Concat([]string{"--add", target}, devices)...)
	if err != nil {
		_, _ = runMdadm("--stop", target) // Try to clean up as best we can
		return fmt.Errorf("Could not add other devices to existing primary %s/%v (%w): %s", existing, devices, err, output)
	}
	return nil
}

func createNewStriped(target string, devices ...string) error {
	output, err := runMdadm(slices.Concat([]string{"--create", target, "--level", "0", "--run", "--raid-devices", fmt.Sprintf("%d", len(devices))}, devices)...)
	if err != nil {
		return fmt.Errorf("Striped raid creation for %s={%v} failed (%w): %s", target, devices, err, output)
	}
	return nil
}

func assembleExistingStriped(target string, devices ...string) error {
	output, err := runMdadm(slices.Concat([]string{"--assemble", target}, devices, []string{"--run"})...)
	if err != nil {
		return fmt.Errorf("Existing assemble failed on %v (%w): %s", devices, err, output)
	}
	return nil
}

func stopAllInactive() error {
	statBytes, err := os.ReadFile(mdstatFile)
	if err != nil {
		return fmt.Errorf("Cannot open %s for stopping inactive: %w", mdstatFile, err)
	}
	inactive_devices := getInactiveDevices(string(statBytes))
	for _, device := range inactive_devices {
		klog.Infof("Stopping inactive device %s", device)
		err := stopRaidDevice(device)
		if err != nil {
			klog.Warningf("Could not stop inactive device %s, continuing anyway: %v", device, err)
		}
	}
	return nil
}

func stopRaidDevice(device string) error {
	if output, err := runMdadm("--stop", device); err != nil {
		return fmt.Errorf("Could not stop %s (%v): %s", device, err, output)
	}
	return nil
}

func getInactiveDevices(mdstats string) []string {
	stats := strings.Split(mdstats, "\n")
	devices := []string{}
	for _, line := range stats {
		matches := mdstatInactive.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		devices = append(devices, fmt.Sprintf("/dev/%s", matches[1]))
	}
	return devices
}

func wipeDevice(device string) error {
	if _, err := os.Stat(device); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Device %s to be wiped does not exist", device)
	}
	_, _ = runMdadm("--zero-superblock", device)
	// There's nothing to recover on errors. If the device was not already an array element, the command will fail.
	return nil
}

func isRaidDevice(device string) error {
	_, err := runMdadm("--detail", device)
	return err // Maybe there's more information to extract from the output?
}

func validateDevice(device string) error {
	info, err := os.Stat(device)
	if err != nil {
		return fmt.Errorf("Could not stat device %s raid: %w", device, err)
	}
	if info.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("Expected %s to be a device", device)
	}
	return nil
}

func isExistingRaidVolume(target, device string) (bool, error) {
	_, err := runMdadm("--examine", device)
	return err == nil, nil
}

func runMdadm(args ...string) (string, error) {
	output, err := util.RunCommand(mdadmCmd, args...)
	return string(output), err
}
