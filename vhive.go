// MIT License
//
// Copyright (c) 2020 Dmitrii Ustiugov, Plamen Petrov and EASE lab
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"flag"
	"github.com/ease-lab/vhive/vcli"
	"math/rand"
	"net"
	"os"
	"runtime"

	ctrdlog "github.com/containerd/containerd/log"
	fccdcri "github.com/ease-lab/vhive/cri"
	ctriface "github.com/ease-lab/vhive/ctriface"
	pb "github.com/ease-lab/vhive/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	port    = ":3333"
)

var (
	flog     *os.File
	orch     *ctriface.Orchestrator

	isSnapshotsEnabled *bool
	isLazyMode         *bool
	isMetricsMode      *bool
	isCliMode          *bool
	criSock            *string
	hostIface          *string
)

func main() {
	var err error

	// limit the number of operating system threads that can execute user-level Go code simultaneously
	runtime.GOMAXPROCS(16)

	rand.Seed(42)

	debug := flag.Bool("dbg", false, "Enable debug logging")

	// CRI arguments
	// Kubernetes sends CRI requests to this socket
	criSock = flag.String("criSock", "/etc/firecracker-containerd/fccd-cri.sock", "Socket address for CRI service")

	// Orch arguments
	snapsCapacityMiB := flag.Int64("snapcapacity", 102400, "Capacity set aside for storing snapshots (Mib)")
	isSparseSnaps := flag.Bool("sparsesnaps", true, "Makes memory files sparse after storing to reduce disk utilization")
	isSnapshotsEnabled = flag.Bool("snapshots", false, "Use VM snapshots when adding function instances")
	isMetricsMode = flag.Bool("metrics", false, "Calculate metrics")
	isLazyMode = flag.Bool("lazy", false, "Enable lazy serving mode when UPFs are enabled")
	hostIface = flag.String("hostIface", "", "Host net-interface for the VMs to bind to for internet access (get default through route if empty)")
	isCliMode = flag.Bool("climode", false, "Host net-interface for the VMs to bind to for internet access (get default through route if empty)")

	// Parse cmd line arguments
	flag.Parse()

	// Setup logging
	if flog, err = os.Create("/tmp/fccd.log"); err != nil {
		panic(err)
	}
	defer flog.Close()

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	//log.SetReportCaller(true) // FXME: make sure it's false unless debugging

	if !*isCliMode {
		log.SetOutput(os.Stdout)
	} else {
		if flog, err = os.Create("/tmp/vhive_log.log"); err != nil {
			panic(err)
		}
		defer flog.Close()
		log.SetOutput(flog)
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging is enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	testModeOn := false

	// Run vHive components
	orch = ctriface.NewOrchestrator(
		*hostIface,
		ctriface.WithTestModeOn(testModeOn),
		ctriface.WithSnapshots(*isSnapshotsEnabled),
		ctriface.WithMetricsMode(*isMetricsMode),
		ctriface.WithLazyMode(*isLazyMode),
	)

	criService, err := fccdcri.NewService(orch, *snapsCapacityMiB, *isSparseSnaps, *isMetricsMode)
	if err != nil {
		log.Fatalf("failed to create CRI service %v", err)
	}

	go criServe(criService)

	if !*isCliMode {
		orchServe()
	} else {
		go orchServe()
		vcli.CreateCli(criService.Coordinator)
	}

}

type server struct {
	pb.UnimplementedOrchestratorServer
}

// Serve K8S CRI requests on specified socket
func criServe(criService *fccdcri.Service) {
	lis, err := net.Listen("unix", *criSock)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	criService.Register(s)


	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Listen to grpc requests to start & stop VMs
func orchServe() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterOrchestratorServer(s, &server{})

	log.Println("Listening on port" + port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}