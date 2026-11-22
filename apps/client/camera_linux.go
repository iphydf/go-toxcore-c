//go:build !android

package main

/*
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <unistd.h>
#include <linux/videodev2.h>

enum { CAMERA_BUFFERS = 4 };

typedef struct {
	int fd;
	int width, height, stride;
	int buffers;
	void *start[CAMERA_BUFFERS];
	size_t length[CAMERA_BUFFERS];
} camera;

static int xioctl(int fd, unsigned long request, void *arg) {
	int r;
	do {
		r = ioctl(fd, request, arg);
	} while (r == -1 && errno == EINTR);
	return r;
}

static void camera_close(camera *c) {
	enum v4l2_buf_type type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	xioctl(c->fd, VIDIOC_STREAMOFF, &type);
	for (int i = 0; i < c->buffers; i++) {
		munmap(c->start[i], c->length[i]);
	}
	close(c->fd);
}

// camera_open starts a device that captures YUYV at about width x height, and
// returns 0, or an errno.
static int camera_open(camera *c, const char *path, int width, int height) {
	memset(c, 0, sizeof *c);
	c->fd = open(path, O_RDWR | O_NONBLOCK | O_CLOEXEC);
	if (c->fd < 0) {
		return errno;
	}
	struct v4l2_capability cap = {0};
	if (xioctl(c->fd, VIDIOC_QUERYCAP, &cap) < 0) {
		int err = errno;
		close(c->fd);
		return err;
	}
	unsigned caps = cap.capabilities & V4L2_CAP_DEVICE_CAPS ? cap.device_caps : cap.capabilities;
	if (!(caps & V4L2_CAP_VIDEO_CAPTURE) || !(caps & V4L2_CAP_STREAMING)) {
		close(c->fd);
		return ENODEV;
	}
	struct v4l2_format fmt = {0};
	fmt.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	fmt.fmt.pix.width = width;
	fmt.fmt.pix.height = height;
	fmt.fmt.pix.pixelformat = V4L2_PIX_FMT_YUYV;
	fmt.fmt.pix.field = V4L2_FIELD_NONE;
	if (xioctl(c->fd, VIDIOC_S_FMT, &fmt) < 0 || fmt.fmt.pix.pixelformat != V4L2_PIX_FMT_YUYV) {
		close(c->fd);
		return ENOTSUP;
	}
	c->width = fmt.fmt.pix.width;
	c->height = fmt.fmt.pix.height;
	c->stride = fmt.fmt.pix.bytesperline;
	struct v4l2_requestbuffers req = {0};
	req.count = CAMERA_BUFFERS;
	req.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	req.memory = V4L2_MEMORY_MMAP;
	if (xioctl(c->fd, VIDIOC_REQBUFS, &req) < 0 || req.count == 0) {
		close(c->fd);
		return ENOMEM;
	}
	for (unsigned i = 0; i < req.count && i < CAMERA_BUFFERS; i++) {
		struct v4l2_buffer buf = {0};
		buf.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
		buf.memory = V4L2_MEMORY_MMAP;
		buf.index = i;
		if (xioctl(c->fd, VIDIOC_QUERYBUF, &buf) < 0) {
			int err = errno;
			camera_close(c);
			return err;
		}
		c->start[i] = mmap(NULL, buf.length, PROT_READ | PROT_WRITE, MAP_SHARED, c->fd, buf.m.offset);
		if (c->start[i] == MAP_FAILED) {
			int err = errno;
			camera_close(c);
			return err;
		}
		c->length[i] = buf.length;
		c->buffers++;
		if (xioctl(c->fd, VIDIOC_QBUF, &buf) < 0) {
			int err = errno;
			camera_close(c);
			return err;
		}
	}
	enum v4l2_buf_type type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	if (xioctl(c->fd, VIDIOC_STREAMON, &type) < 0) {
		int err = errno;
		camera_close(c);
		return err;
	}
	return 0;
}

// camera_next waits up to timeout ms for a picture, and returns its buffer's
// index, -1 if none came, or -2 with errno set.
static int camera_next(camera *c, int timeout, size_t *used) {
	struct pollfd p = {.fd = c->fd, .events = POLLIN};
	int r = poll(&p, 1, timeout);
	if (r < 0) {
		return errno == EINTR ? -1 : -2;
	}
	if (r == 0) {
		return -1;
	}
	struct v4l2_buffer buf = {0};
	buf.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	buf.memory = V4L2_MEMORY_MMAP;
	if (xioctl(c->fd, VIDIOC_DQBUF, &buf) < 0) {
		return errno == EAGAIN ? -1 : -2;
	}
	*used = buf.bytesused;
	return buf.index;
}

static int camera_release(camera *c, int index) {
	struct v4l2_buffer buf = {0};
	buf.type = V4L2_BUF_TYPE_VIDEO_CAPTURE;
	buf.memory = V4L2_MEMORY_MMAP;
	buf.index = index;
	return xioctl(c->fd, VIDIOC_QBUF, &buf) < 0 ? errno : 0;
}
*/
import "C"

import (
	"errors"
	"io"
	"log"
	"path/filepath"
	"syscall"
	"unsafe"
)

// v4l2Camera captures from a webcam through Video4Linux, on a goroutine.
type v4l2Camera struct {
	cam     C.camera
	done    chan struct{}
	stopped chan struct{}
}

func (pulseMedia) openCamera(frame func(width, height int, y, u, v []byte)) (io.Closer, error) {
	devices, _ := filepath.Glob("/dev/video*")
	c := &v4l2Camera{done: make(chan struct{}), stopped: make(chan struct{})}
	opened := false
	for _, dev := range devices {
		path := C.CString(dev)
		err := C.camera_open(&c.cam, path, 640, 480)
		C.free(unsafe.Pointer(path))
		if err == 0 {
			opened = true
			break
		}
	}
	if !opened {
		return nil, errors.New("there's no camera the app can use")
	}
	go c.capture(frame)
	return c, nil
}

func (c *v4l2Camera) capture(frame func(width, height int, y, u, v []byte)) {
	defer close(c.stopped)
	// I420 wants even sizes: drop an odd last column or row.
	w, h, stride := int(c.cam.width)&^1, int(c.cam.height)&^1, int(c.cam.stride)
	var y, u, v []byte
	for {
		select {
		case <-c.done:
			return
		default:
		}
		var used C.size_t
		i, err := C.camera_next(&c.cam, 200, &used)
		switch {
		case i == -1:
			continue
		case i < 0:
			log.Printf("camera: %v", err)
			return
		}
		if int(used) >= stride*h {
			data := unsafe.Slice((*byte)(c.cam.start[i]), int(used))
			y, u, v = yuyvToI420(data, w, h, stride, y, u, v)
			frame(w, h, y, u, v)
		}
		if err := C.camera_release(&c.cam, i); err != 0 {
			log.Printf("camera: %v", syscall.Errno(err))
			return
		}
	}
}

func (c *v4l2Camera) Close() error {
	close(c.done)
	<-c.stopped
	C.camera_close(&c.cam)
	return nil
}

// yuyvToI420 converts a YUYV picture of even size, with stride bytes a row,
// into the planes, which it makes if they are too small. Each chroma sample is
// the average of the two rows it covers.
func yuyvToI420(src []byte, w, h, stride int, y, u, v []byte) ([]byte, []byte, []byte) {
	y, u, v = grow(y, w*h), grow(u, w*h/4), grow(v, w*h/4)
	for j := range h {
		row := src[j*stride:]
		for i := range w {
			y[j*w+i] = row[2*i]
		}
	}
	for j := range h / 2 {
		top, bottom := src[2*j*stride:], src[(2*j+1)*stride:]
		for i := range w / 2 {
			// Each 4 bytes are Y0 U Y1 V, for two pixels.
			u[j*w/2+i] = byte((int(top[4*i+1]) + int(bottom[4*i+1]) + 1) / 2)
			v[j*w/2+i] = byte((int(top[4*i+3]) + int(bottom[4*i+3]) + 1) / 2)
		}
	}
	return y, u, v
}
