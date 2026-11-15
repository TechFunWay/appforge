package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/TechFunWay/appforge/kit/manifest"
)

// 用 medcare 的真实参数渲染整套 fnpack 文件，与 medcare/fnpack/ 逐字节比对。
func main() {
	m := &manifest.Manifest{
		Name: "medcare", DisplayName: "用药管理", Version: "0.1.37",
		Desc:  "面向用药人与监护人的家庭协作用药本：药品批次库存、用药计划、到点提醒与打卡、处方单 AI 识别、邮件/钉钉/飞书通知与依从性统计",
		Port:  8913,
		Intro: "用药管理是家庭协作用药本：支持多用药本、邀请成员、药品库存、用药计划、打卡调整、处方 AI 识别与邮件/钉钉/飞书提醒。安装完成后，首次访问并注册的用户将成为管理员。运行数据（数据库、上传文件、日志）保存在应用数据目录中。",
	}
	files, err := m.RenderFnpackFiles()
	if err != nil {
		fmt.Println("渲染失败:", err)
		os.Exit(2)
	}
	refDir := os.Args[1]
	bad := 0
	for _, name := range []string{"cmd/main", "cmd/config_callback", "cmd/config_init", "cmd/install_callback", "cmd/install_init", "cmd/port_config", "cmd/uninstall_callback", "cmd/uninstall_init", "cmd/upgrade_callback", "cmd/upgrade_init", "config/privilege", "config/resource", "app/ui/config", "wizard/config", "wizard/install", "wizard/uninstall", "wizard/upgrade"} {
		want, err := os.ReadFile(filepath.Join(refDir, name))
		if err != nil {
			fmt.Println("读金样失败", name, err)
			bad++
			continue
		}
		got, ok := files[name]
		if !ok {
			fmt.Printf("缺模板  %s\n", name)
			bad++
			continue
		}
		if string(got.Data) == string(want) {
			fmt.Printf("一致    %s\n", name)
			continue
		}
		bad++
		fmt.Printf("漂移    %s（金样 %d 字节 / 渲染 %d 字节）\n", name, len(want), len(got.Data))
		// 打印第一处差异上下文
		a, b := []byte(want), got.Data
		for i := 0; i < len(a) && i < len(b); i++ {
			if a[i] != b[i] {
				lo := i - 40
				if lo < 0 {
					lo = 0
				}
				fmt.Printf("   首差 @%d 金样: %q\n            渲染: %q\n", i, string(a[lo:min(i+40, len(a))]), string(b[lo:min(i+40, len(b))]))
				break
			}
		}
	}
	fmt.Println("漂移文件数:", bad)
	if bad > 0 {
		os.Exit(1)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
