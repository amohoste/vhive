package ctrimages

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/remotes/docker"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type ImageState struct {
	sync.Mutex
	pulled bool
}

func NewImageState() *ImageState {
	log.Info("Creating network manager")
	state := new(ImageState)
	state.pulled = false
	return state
}


type ImageManager struct {
	sync.Mutex
	snapshotter  string						 // image snapshotter
	cachedImages map[string]*containerd.Image // Cached container images
	imageStates  map[string]*ImageState
	client       *containerd.Client
}

func NewImageManager(client *containerd.Client, snapshotter string) *ImageManager {
	log.Info("Creating network manager")
	manager := new(ImageManager)
	manager.snapshotter = snapshotter
	manager.cachedImages = make(map[string]*containerd.Image)
	manager.imageStates = make(map[string]*ImageState)
	manager.client = client
	return manager
}

func (mgr *ImageManager) pullImage(ctx context.Context, imageName string) (*containerd.Image, error) {
	var err error
	var image containerd.Image
	log.Debug(fmt.Sprintf("Pulling image %s", imageName))

	imageURL := getImageURL(imageName)
	local, _ := isLocalDomain(imageURL)
	if local {
		// Pull local image using HTTP
		resolver := docker.NewResolver(docker.ResolverOptions{
			Client: http.DefaultClient,
			Hosts: docker.ConfigureDefaultRegistries(
				docker.WithPlainHTTP(docker.MatchAllHosts),
			),
		})
		image, err = mgr.client.Pull(ctx, imageURL,
			containerd.WithPullUnpack,
			containerd.WithPullSnapshotter(mgr.snapshotter),
			containerd.WithResolver(resolver),
		)
	} else {
		// Pull remote image
		image, err = mgr.client.Pull(ctx, imageURL,
			containerd.WithPullUnpack,
			containerd.WithPullSnapshotter(mgr.snapshotter),
		)
	}
	if err != nil {
		return nil, err
	}
	mgr.Lock()
	mgr.cachedImages[imageName] = &image
	mgr.Unlock()
	return &image, nil
}

func (mgr *ImageManager) GetImage(ctx context.Context, imageName string) (*containerd.Image, error) {
	var image *containerd.Image
	var err error

	mgr.Lock()
	imgState, found := mgr.imageStates[imageName]
	if !found {
		mgr.imageStates[imageName] = NewImageState()
	}
	mgr.Unlock()

	imgState.Lock()
	if !imgState.pulled {
		image, err = mgr.pullImage(ctx, imageName)
		if err != nil {
			imgState.Unlock()
			return nil, err
		}
		imgState.pulled = true
		imgState.Unlock()
	} else {
		imgState.Unlock()
		mgr.Lock()
		image = mgr.cachedImages[imageName]
		mgr.Unlock()
	}

	return image, nil
}

// Converts an image name to a url if it is not a URL
func getImageURL(image string) string {
	// Pull from dockerhub by default if not specified (default k8s behavior)
	if strings.Contains(image, ".") {
		return image
	}
	return "docker.io/" + image

}

// Checks whether a URL has a .local domain
func isLocalDomain(s string) (bool, error) {
	if ! strings.Contains(s, "://") {
		s = "dummy://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return false, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	i := strings.LastIndex(host, ".")
	tld := host[i+1:]

	return tld == "local", nil
}