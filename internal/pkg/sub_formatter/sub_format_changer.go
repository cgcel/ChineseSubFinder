package sub_formatter

import (
	"errors"
	"fmt"
	"github.com/allanpk716/ChineseSubFinder/internal/ifaces"
	movieHelper "github.com/allanpk716/ChineseSubFinder/internal/logic/movie_helper"
	seriesHelper "github.com/allanpk716/ChineseSubFinder/internal/logic/series_helper"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/sub_formatter/common"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/sub_formatter/emby"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/sub_formatter/normal"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/sub_helper"
	"github.com/allanpk716/ChineseSubFinder/internal/types"
	"os"
	"strings"
)

type SubFormatChanger struct {
	movieRootDir  string
	seriesRootDir string
	formatter     map[string]ifaces.ISubFormatter
}

func NewSubFormatChanger(movieRootDir string, seriesRootDir string) *SubFormatChanger {

	formatter := SubFormatChanger{movieRootDir: movieRootDir, seriesRootDir: seriesRootDir}
	// 初始化支持的 formatter
	// normal
	formatter.formatter = make(map[string]ifaces.ISubFormatter)
	normalM := normal.NewFormatter()
	formatter.formatter[normalM.GetFormatterName()] = normalM
	// emby
	embyM := emby.NewFormatter()
	formatter.formatter[embyM.GetFormatterName()] = embyM
	return &formatter
}

// AutoDetectThenChangeTo 自动检测字幕的命名格式，然后转换到目标的 formatter 上
func (s SubFormatChanger) AutoDetectThenChangeTo(desFormatter common.FormatterName) (RenameResults, error) {

	var err error
	outStruct := RenameResults{}
	outStruct.RenamedFiles = make(map[string]int)
	outStruct.ErrFiles = make(map[string]int)
	if pkg.IsDir(s.movieRootDir) == false {
		return outStruct, errors.New("movieRootDir path not exist: " + s.movieRootDir)
	}
	if pkg.IsDir(s.seriesRootDir) == false {
		return outStruct, errors.New("seriesRootDir path not exist: " + s.seriesRootDir)
	}
	// 先找出有那些电影文件夹和连续剧文件夹
	var movieFullPathList = make([]string, 0)
	movieFullPathList, err = pkg.SearchMatchedVideoFile(s.movieRootDir)
	if err != nil {
		return outStruct, err
	}
	seriesDirList, err := seriesHelper.GetSeriesList(s.seriesRootDir)
	if err != nil {
		return outStruct, err
	}
	// 搜索所有的字幕，找到相关的字幕进行修改
	for _, one := range movieFullPathList {
		found := false
		var fitMovieNameSubList = make([]string, 0)
		found, _, fitMovieNameSubList, err = movieHelper.MovieHasChineseSub(one)
		if err != nil || found == false {
			continue
		}
		// 判断是否是符合要求
		for _, fitSubName := range fitMovieNameSubList {
			s.autoDetectAndChange(&outStruct, fitSubName, desFormatter)
		}
	}
	// 连续剧
	var seriesSubFiles = make([]string, 0)
	for _, oneSeriesDir := range seriesDirList {
		seriesSubFiles, err = sub_helper.SearchMatchedSubFile(oneSeriesDir)
		if err != nil {
			return outStruct, err
		}
		// 判断是否是符合要求
		for _, fitSubName := range seriesSubFiles {
			s.autoDetectAndChange(&outStruct, fitSubName, desFormatter)
		}
	}

	return outStruct, nil
}

// autoDetectAndChange 自动检测命名格式，然后修改至目标的命名格式
func (s SubFormatChanger) autoDetectAndChange(outStruct *RenameResults, fitSubName string, desFormatter common.FormatterName) {

	for _, formatter := range s.formatter {
		bok, fileNameWithOutExt, subExt, subLang, extraSubPreName := formatter.IsMatchThisFormat(fitSubName)
		if bok == false {
			continue
		}
		// 这里得到的 subExt 可能是 .ass or .default.ass or .forced.ass
		// 需要进行剔除，因为后续的 GenerateMixSubName 会自动生成对应的附加后缀名
		// 转换格式后，需要保留之前的 default 或者 forced
		findDefault := false
		findForce := false
		if strings.Contains(subExt, types.Sub_Ext_Mark_Default) == true {
			subExt = strings.Replace(subExt, types.Sub_Ext_Mark_Default, "", -1)
			findDefault = true
		}
		if strings.Contains(subExt, types.Sub_Ext_Mark_Forced) == true {
			subExt = strings.Replace(subExt, types.Sub_Ext_Mark_Forced, "", -1)
			findForce = true
		}
		// 通过传入的目标格式化 formatter 的名称去调用
		newSubFileName := ""
		newName, newDefaultName, newForcedName := s.formatter[fmt.Sprintf("%s", desFormatter)].
			GenerateMixSubNameBase(fileNameWithOutExt, subExt, subLang, extraSubPreName)
		if findDefault == false && findForce == false {
			// 使用没得额外 Default 或者 Forced 的名称即可
			newSubFileName = newName
		} else if findDefault == true {
			newSubFileName = newDefaultName
		} else if findForce == true {
			newSubFileName = newForcedName
		}
		if newSubFileName == "" {
			continue
		}
		// 确认改格式
		err := os.Rename(fitSubName, newSubFileName)
		if err != nil {
			tmpName := pkg.FixWindowPathBackSlash(fitSubName)
			outStruct.ErrFiles[tmpName] += 1
			continue
		} else {
			tmpName := pkg.FixWindowPathBackSlash(newSubFileName)
			outStruct.RenamedFiles[tmpName] += 1
		}
	}
}

type RenameResults struct {
	RenamedFiles map[string]int
	ErrFiles     map[string]int
}

// GetSubFormatter 选择字幕命名格式化的实例
func GetSubFormatter(subNameFormatter int) ifaces.ISubFormatter {
	var subFormatter ifaces.ISubFormatter
	switch subNameFormatter {
	case int(common.Emby):
		{
			subFormatter = emby.NewFormatter()
			break
		}
	case int(common.Normal):
		{
			subFormatter = normal.NewFormatter()
			break
		}
	default:
		{
			subFormatter = emby.NewFormatter()
			break
		}
	}

	return subFormatter
}
