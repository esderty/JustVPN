package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"embed"
)

//go:embed sing-box.exe
//go:embed wintun.dll
var embeddedFiles embed.FS

type App struct {
	ctx         context.Context
	sbProcess   *os.Process
	vlessLink   string
	configPath  string
	sbPath      string
	wintunPath  string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.Disconnect()
	a.cleanup()
}

func (a *App) cleanup() {
	if a.sbPath != "" {
		os.Remove(a.sbPath)
	}
	if a.wintunPath != "" {
		os.Remove(a.wintunPath)
	}
}

func getAppConfigPath(fileName string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appConfigDir := filepath.Join(configDir, "JustVPNApp")
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appConfigDir, fileName), nil
}

func (a *App) getDeviceID() (string, error) {
	filePath, err := getAppConfigPath("device.id")
	if err != nil {
		return "", err
	}
	idBytes, err := ioutil.ReadFile(filePath)
	if err == nil {
		return string(idBytes), nil
	}
	newID := uuid.New().String()
	if err := ioutil.WriteFile(filePath, []byte(newID), 0644); err != nil {
		return "", err
	}
	return newID, nil
}

func (a *App) SaveKey(key string) error {
	filePath, err := getAppConfigPath("auth.key")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, []byte(key), 0644)
}

func (a *App) LoadKey() (string, error) {
	filePath, err := getAppConfigPath("auth.key")
	if err != nil {
		return "", err
	}
	keyBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(keyBytes), nil
}

func (a *App) DeleteKey() error {
	filePath, err := getAppConfigPath("auth.key")
	if err != nil {
		return err
	}
	return os.Remove(filePath)
}

func (a *App) OpenURL(url string) {
	wailsruntime.BrowserOpenURL(a.ctx, url)
}

func (a *App) ActivateWithKey(key string) (string, error) {
	deviceID, err := a.getDeviceID()
	if err != nil {
		return "", err
	}
	authServerURL := "http://82.115.6.90:8080/login"
	requestBody, _ := json.Marshal(map[string]string{"auth_code": key, "device_id": deviceID})
	resp, err := http.Post(authServerURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(string(body))
	}
	a.vlessLink = string(body)
	return a.vlessLink, nil
}

// Connect запускает sing-box в режиме TUN с DPI bypass (fragment) и packet_encoding: xudp
func (a *App) Connect(vlessLink string) (string, error) {
	if a.sbProcess != nil {
		return "Already connected", nil
	}
	a.vlessLink = vlessLink
	parsedURL, err := url.Parse(a.vlessLink)
	if err != nil {
		return "", fmt.Errorf("failed to parse vless link: %w", err)
	}
	userID := parsedURL.User.Username()
	address := parsedURL.Hostname()
	port, _ := strconv.Atoi(parsedURL.Port())
	queryParams := parsedURL.Query()
	sni := queryParams.Get("sni")
	publicKey := queryParams.Get("pbk")
	fingerprint := queryParams.Get("fp")
	shortID := queryParams.Get("sid")

	// Формируем конфиг sing-box
	sbConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
			"output": "",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type": "tun",
				"tag": "tun-in",
				"interface_name": "JustVPN",
				"inet4_address": "10.66.66.2/30",
				"mtu": 9000,
				"auto_route": true,
				"strict_route": false,
				"stack": "system",
				"gateway": "10.66.66.1",
				"dns": map[string]interface{}{
					"servers": []string{"1.1.1.1", "8.8.8.8"},
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"type": "vless",
				"tag": "vless-out",
				"server": address,
				"server_port": port,
				"uuid": userID,
				"flow": "xtls-rprx-vision",
				"packet_encoding": "xudp",
				"tls": map[string]interface{}{
					"enabled": true,
					"server_name": sni,
				},
				"reality": map[string]interface{}{
					"enabled": true,
					"public_key": publicKey,
					"short_id": shortID,
					"fingerprint": fingerprint,
				},
				"transport": map[string]interface{}{
					"type": "tcp",
					"fragment": map[string]interface{}{
						"enabled": true,
						"length": 900,
						"interval": 10,
					},
				},
			},
			map[string]interface{}{
				"type": "direct",
				"tag": "direct",
			},
			map[string]interface{}{
				"type": "block",
				"tag": "block",
			},
		},
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"rules": []interface{}{},
		},
	}

	configBytes, err := json.MarshalIndent(sbConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal sing-box config: %w", err)
	}

	configPath, err := getAppConfigPath("singbox-config.json")
	if err != nil {
		return "", fmt.Errorf("failed to get config path: %w", err)
	}
	a.configPath = configPath
	if err := ioutil.WriteFile(a.configPath, configBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	// Извлекаем sing-box.exe и wintun.dll в AppData
	sbPath, err := getAppConfigPath("sing-box.exe")
	if err != nil {
		return "", fmt.Errorf("failed to get sing-box.exe path: %w", err)
	}
	wintunPath, err := getAppConfigPath("wintun.dll")
	if err != nil {
		return "", fmt.Errorf("failed to get wintun.dll path: %w", err)
	}
	sbBytes, err := embeddedFiles.ReadFile("sing-box.exe")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded sing-box.exe: %w", err)
	}
	wintunBytes, err := embeddedFiles.ReadFile("wintun.dll")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded wintun.dll: %w", err)
	}
	if err := ioutil.WriteFile(sbPath, sbBytes, 0755); err != nil {
		return "", fmt.Errorf("failed to write sing-box.exe: %w", err)
	}
	if err := ioutil.WriteFile(wintunPath, wintunBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write wintun.dll: %w", err)
	}
	a.sbPath = sbPath
	a.wintunPath = wintunPath

	cmd := exec.Command(a.sbPath, "run", "-c", a.configPath)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	cmd.Dir = filepath.Dir(a.sbPath)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start sing-box: %w", err)
	}
	a.sbProcess = cmd.Process

	return "Connected (TUN mode)", nil
}

func (a *App) Disconnect() (string, error) {
	if a.sbProcess == nil {
		return "Not connected", nil
	}
	if runtime.GOOS == "windows" {
		pid := a.sbProcess.Pid
		cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid), "/T")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Run()
	} else {
		a.sbProcess.Kill()
	}
	a.sbProcess = nil
	if a.configPath != "" {
		os.Remove(a.configPath)
	}
	a.cleanup()
	return "Disconnected", nil
}

func (a *App) GetStatus() bool {
	return a.sbProcess != nil
}