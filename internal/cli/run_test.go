package cli

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/sirrobot01/archbench"
)

func TestSelectTargets(t *testing.T) {
	targets := []archbench.Target{
		{Name: "local"},
		{Name: "amd64"},
	}

	if got := selectTargets(targets, ""); !reflect.DeepEqual(got, targets) {
		t.Fatalf("select all = %#v, want %#v", got, targets)
	}

	got := selectTargets(targets, "amd64")
	if len(got) != 1 || got[0].Name != "amd64" {
		t.Fatalf("select amd64 = %#v", got)
	}
}

func TestRunTargetsBoundsConcurrency(t *testing.T) {
	targets := []archbench.Target{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	}
	started := make(chan string, len(targets))
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := runTargets(context.Background(), targets, 2, func(_ context.Context, target archbench.Target) (*targetRun, error) {
			started <- target.Name
			<-release
			return &targetRun{Result: resultFor(target.Name)}, nil
		})
		done <- err
	}()

	_ = receiveStarted(t, started)
	_ = receiveStarted(t, started)
	select {
	case name := <-started:
		t.Fatalf("target %q started before concurrency slot was released", name)
	case <-time.After(50 * time.Millisecond):
	}

	release <- struct{}{}
	_ = receiveStarted(t, started)
	release <- struct{}{}
	release <- struct{}{}

	if err := <-done; err != nil {
		t.Fatalf("runTargets: %v", err)
	}
}

func TestRunTargetsReturnsResultsInTargetOrder(t *testing.T) {
	targets := []archbench.Target{
		{Name: "slow"},
		{Name: "fast"},
		{Name: "middle"},
	}

	results, err := runTargets(context.Background(), targets, 3, func(_ context.Context, target archbench.Target) (*targetRun, error) {
		switch target.Name {
		case "slow":
			time.Sleep(30 * time.Millisecond)
		case "middle":
			time.Sleep(10 * time.Millisecond)
		}
		return &targetRun{Result: resultFor(target.Name)}, nil
	})
	if err != nil {
		t.Fatalf("runTargets: %v", err)
	}

	got := make([]string, 0, len(results))
	for _, result := range results {
		got = append(got, result.Target.Name)
	}
	want := []string{"slow", "fast", "middle"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result order = %#v, want %#v", got, want)
	}
}

func receiveStarted(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case name := <-ch:
		return name
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for target to start")
		return ""
	}
}

func resultFor(target string) *archbench.RunResult {
	return &archbench.RunResult{
		Target: target,
		Mode:   archbench.ModeBench,
		Metadata: archbench.Metadata{
			Arch: "test",
			OS:   "test",
		},
	}
}
