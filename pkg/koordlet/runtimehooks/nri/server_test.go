package nri

import (
	"github.com/containerd/nri/pkg/api"
	"testing"
)

func TestName(t *testing.T) {
	api.ParseEventMask("RunPodSandbox,StartContainer,UpdateContainer")
}
