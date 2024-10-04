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

package e2e

import (
	"context"
	"strings"
	"testing"
)

func TestTmpfsSetup(t *testing.T) {
	skipUnlessLabeled(t, "tmpfs")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()

	restartDriver(ctx, t)

	pod := startCachePod(ctx, t, "mark", "tmpfs")
	node := pod.Spec.NodeName
	if _, err := runOnPod(ctx, t, pod, "touch", "/cache/mark"); err != nil {
		t.Fatalf("Could not touch cache: %v", err)
	}
	if out, err := runOnPod(ctx, t, pod, "ls", "/cache/mark"); err != nil || !strings.Contains(out, "/cache/mark") {
		t.Fatalf("Mark didn't stick: %s / %v", out, err)
	}
	deletePod(ctx, t, pod)
	pod = startCachePodOnNode(ctx, t, "check", node)
	if out, err := runOnPod(ctx, t, pod, "ls", "/cache/mark"); err != nil || !strings.Contains(out, "/cache/mark") {
		t.Fatalf("Could not verify mark: %s / %v", out, err)
	}
	deletePod(ctx, t, pod)
}

func TestTmpfsDriverRestart(t *testing.T) {
	skipUnlessLabeled(t, "tmpfs")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()

	restartDriver(ctx, t)

	p1 := startCachePod(ctx, t, "p1", "tmpfs")
	node := p1.Spec.NodeName
	if _, err := runOnPod(ctx, t, p1, "touch", "/cache/m1"); err != nil {
		t.Fatalf("Could not touch 1: %v", err)
	}

	restartDriver(ctx, t)

	p2 := startCachePodOnNode(ctx, t, "p2", node)
	if out, err := runOnPod(ctx, t, p2, "ls", "/cache/m1"); err == nil {
		t.Fatalf("Unexpected mark found after restart: %s", out)
	}
	if out, err := runOnPod(ctx, t, p1, "ls", "/cache/m1"); err != nil || !strings.Contains(out, "/cache/m1") {
		t.Fatalf("Mark 1 disappeared after restart: %s", out)
	}

	if _, err := runOnPod(ctx, t, p2, "touch", "/cache/m2"); err != nil {
		t.Fatalf("Could not touch m2: %v", err)
	}
	p3 := startCachePodOnNode(ctx, t, "p3", node)
	if out, err := runOnPod(ctx, t, p2, "ls", "/cache/m2"); err != nil || !strings.Contains(out, "/cache/m2") {
		t.Fatalf("Could not find m2: %s", out)
	}

	deletePod(ctx, t, p1)
	deletePod(ctx, t, p2)
	deletePod(ctx, t, p3)
}
