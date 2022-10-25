package settings

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/allanpk716/ChineseSubFinder/pkg/strcut_json"
)

type Settings struct {
	SpeedDevMode          bool // 是否为开发模式，代码开启这个会跳过某些流程，加快测试速度
	configFPath           string
	UserInfo              *UserInfo              `json:"user_info"`
	CommonSettings        *CommonSettings        `json:"common_settings"`
	SubtitleSources       *SubtitleSources       `json:"subtitle_sources"`
	AdvancedSettings      *AdvancedSettings      `json:"advanced_settings"`
	EmbySettings          *EmbySettings          `json:"emby_settings"`
	DeveloperSettings     *DeveloperSettings     `json:"developer_settings"`
	TimelineFixerSettings *TimelineFixerSettings `json:"timeline_fixer_settings"`
	ExperimentalFunction  *ExperimentalFunction  `json:"experimental_function"`
}

// Get 获取 Settings 的实例
func Get(reloadSettings ...bool) *Settings {

	_settingsLocker.Lock()
	defer _settingsLocker.Unlock()

	if _settings == nil {

		_settingsOnce.Do(func() {

			if _configRootPath == "" {
				panic("请先调用 SetConfigRootPath 设置配置文件的根目录")
			}
			_settings = NewSettings(_configRootPath)
			if isFile(_settings.configFPath) == false {

				err := os.MkdirAll(filepath.Dir(_settings.configFPath), os.ModePerm)
				if err != nil {
					panic("创建配置文件目录失败，" + err.Error())
				}
				// 配置文件不存在，新建一个空白的
				err = _settings.Save()
				if err != nil {
					panic("Can't Save Config File:" + configName + " Error: " + err.Error())
				}
			} else {
				// 读取存在的文件
				err := _settings.read()
				if err != nil {
					panic("Can't Read Config File:" + configName + " Error: " + err.Error())
				}
			}
		})
		// 是否需要重新读取配置信息，这个可能在每次保存配置文件后需要操作
		if len(reloadSettings) >= 1 {
			if reloadSettings[0] == true {
				err := _settings.read()
				if err != nil {
					panic("Can't Read Config File:" + configName + " Error: " + err.Error())
				}
			}
		}

	}
	return _settings
}

// SetFullNewSettings 从 Web 端传入新的 Settings 完整设置
func SetFullNewSettings(inSettings *Settings) error {

	_settingsLocker.Lock()
	defer _settingsLocker.Unlock()

	// 保存前进行本地代理的关闭
	err := _settings.AdvancedSettings.ProxySettings.CloseLocalHttpProxyServer()
	if err != nil {
		return err
	}
	nowConfigFPath := _settings.configFPath
	_settings = inSettings
	_settings.configFPath = nowConfigFPath

	return _settings.Save()
}

// SetConfigRootPath 需要先设置这个信息再调用 Get
func SetConfigRootPath(configRootPath string) {
	_configRootPath = configRootPath
}

func NewSettings(configRootDirFPath string) *Settings {

	nowConfigFPath := filepath.Join(configRootDirFPath, configName)

	return &Settings{
		configFPath:           nowConfigFPath,
		UserInfo:              &UserInfo{},
		CommonSettings:        NewCommonSettings(),
		SubtitleSources:       NewSubtitleSources(),
		AdvancedSettings:      NewAdvancedSettings(),
		EmbySettings:          NewEmbySettings(),
		DeveloperSettings:     NewDeveloperSettings(),
		TimelineFixerSettings: NewTimelineFixerSettings(),
		ExperimentalFunction:  NewExperimentalFunction(),
	}
}

func (s *Settings) read() error {

	// 需要检查 url 是否正确
	newEmbyAddressUrl := removeSuffixAddressSlash(s.EmbySettings.AddressUrl)
	_, err := url.Parse(newEmbyAddressUrl)
	if err != nil {
		return err
	}
	// 因为是重新加载配置文件信息，所以需要考虑提前关闭之前开启的本地代理
	err = s.AdvancedSettings.ProxySettings.CloseLocalHttpProxyServer()
	if err != nil {
		return err
	}
	err = strcut_json.ToStruct(s.configFPath, s)
	if err != nil {
		return err
	}
	s.EmbySettings.AddressUrl = newEmbyAddressUrl

	return nil
}

func (s *Settings) Save() error {

	// 需要检查 url 是否正确
	newEmbyAddressUrl := removeSuffixAddressSlash(s.EmbySettings.AddressUrl)
	_, err := url.Parse(newEmbyAddressUrl)
	if err != nil {
		return err
	}
	err = s.AdvancedSettings.ProxySettings.CloseLocalHttpProxyServer()
	if err != nil {
		return err
	}
	s.EmbySettings.AddressUrl = newEmbyAddressUrl

	return strcut_json.ToFile(s.configFPath, s)
}

func (s *Settings) GetNoPasswordSettings() *Settings {

	nowSettings := NewSettings(_configRootPath)
	err := nowSettings.read()
	if err != nil {
		panic("Can't Read Config File:" + configName + " Error: " + err.Error())
	}
	// 需要关闭本地代理的实例，否则无法进行 clone 操作
	//_ = s.AdvancedSettings.ProxySettings.CloseLocalHttpProxyServer()
	//nowSettings := clone.Clone(s).(*Settings)
	nowSettings.UserInfo.Password = noPassword4Show
	return nowSettings
}

// Check 检测，某些参数有范围限制
func (s *Settings) Check() {

	// 每个网站最多找 Top 几的字幕结果，评价系统成熟后，才有设计的意义
	if s.AdvancedSettings.Topic < 0 || s.AdvancedSettings.Topic > 3 {
		s.AdvancedSettings.Topic = 1
	}
	// 如果 Debug 模式开启了，强制设置线程数为1，方便定位问题
	if s.AdvancedSettings.DebugMode == true {
		s.CommonSettings.Threads = 1
	} else {
		// 并发线程的范围控制
		if s.CommonSettings.Threads <= 0 || s.CommonSettings.Threads > 6 {
			s.CommonSettings.Threads = 6
		}
	}
}

// isDir 存在且是文件夹
func isDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// isFile 存在且是文件
func isFile(filePath string) bool {
	s, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

// 将字符串后面最后一个字符，如果是 / 那么则替换掉，多个也会
func removeSuffixAddressSlash(orgAddressUrlString string) string {

	outString := orgAddressUrlString

	for {
		if strings.HasSuffix(outString, "/") == true {
			outString = outString[:len(outString)-1]
		} else {
			break
		}
	}
	return outString
}

var (
	_settings       *Settings
	_settingsLocker sync.Mutex
	_settingsOnce   sync.Once
	_configRootPath string
)

const (
	noPassword4Show = "******" // 填充使用
	configName      = "ChineseSubFinderSettings.json"
)
