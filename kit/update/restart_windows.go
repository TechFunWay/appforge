//go:build windows

package update

// windows 平台不支持原地自更新：运行中的 exe 无法替换，请下载新版
// zip 覆盖安装。这里只提供占位实现保证跨平台编译。

import (
	"errors"
	"syscall"
)

type syscallStatfsT struct{}

func statfs(string, *syscallStatfsT) error {
	return errors.New("windows 不支持磁盘余量查询")
}

func (st *syscallStatfsT) freeBytes() uint64 { return 0 }

func restartProcess() error {
	return errors.New("windows 不支持原地自更新，请下载新版 zip 覆盖安装")
}

var _ = syscall.Getpagesize
