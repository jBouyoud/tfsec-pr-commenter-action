package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-github/v32/github"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var patchRegex, _ = regexp.Compile("^@@.*\\+(\\d+),(\\d+).+?@@")
var commitRefRegex, _ = regexp.Compile(".+ref=(.+)")

type commitFileInfo struct {
	fileName  string
	hunkStart int
	hunkEnd   int
	sha       string
}

type commentBlock struct {
	fileName    string
	startLine   int
	endLine     int
	position    int
	sha         string
	code        string
	description string
	provider    string
}

type checkRange struct {
	Filename  string `json:"filename"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type result struct {
	RuleID          string      `json:"rule_id"`
	RuleDescription string      `json:"rule_description"`
	RuleProvider    string      `json:"rule_provider"`
	Link            string      `json:"link"`
	Range           *checkRange `json:"location"`
	Description     string      `json:"description"`
	RangeAnnotation string      `json:"-"`
	Severity        string      `json:"severity"`
}

func loadResultsFile() ([]*result, error) {
	results := struct{ Results []*result }{}

	file, err := ioutil.ReadFile("results.json")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(file, &results)
	if err != nil {
		return nil, err
	}
	return results.Results, nil
}

func getRelevantResults(prFiles []*github.CommitFile) ([]*commentBlock, error) {
	workspacePath := fmt.Sprintf("%s/", os.Getenv("GITHUB_WORKSPACE"))
	results, err := loadResultsFile()
	if err != nil {
		return nil, err
	}

	var commentBlocks []*commentBlock
	for _, result := range results {
		for _, file := range prFiles {
			result.Range.Filename = strings.ReplaceAll(result.Range.Filename, workspacePath, "")
			if result.Range.Filename == *file.Filename {
				info, err := getCommitInfo(file)
				if err != nil {
					return nil, err
				}

				if shouldInclude(result, info) {
					commentBlock := &commentBlock{
						fileName:    info.fileName,
						startLine:   result.Range.StartLine,
						endLine:     result.Range.EndLine,
						position:    result.Range.StartLine - info.hunkStart,
						sha:         info.sha,
						code:        result.RuleID,
						description: result.Description,
						provider:    result.RuleProvider,
					}

					commentBlocks = append(commentBlocks, commentBlock)
				}
			}
		}
	}
	return commentBlocks, nil
}

func shouldInclude(result *result, info *commitFileInfo) bool {
	return result.Range.StartLine < (info.hunkStart+info.hunkEnd) && result.Range.StartLine > info.hunkStart
}

func getCommitInfo(file *github.CommitFile) (*commitFileInfo, error) {
	groups := patchRegex.FindAllStringSubmatch(file.GetPatch(), -1)
	if len(groups) < 1 {
		return nil, errors.New("the patch details could not be resolved")
	}
	hunkStart, _ := strconv.Atoi(groups[0][1])
	hunkEnd, _ := strconv.Atoi(groups[0][2])

	shaGroups := commitRefRegex.FindAllStringSubmatch(file.GetContentsURL(), -1)
	if len(shaGroups) < 1 {
		return nil, errors.New("the sha details could not be resolved")
	}
	sha := shaGroups[0][1]

	return &commitFileInfo{
		fileName:  *file.Filename,
		hunkStart: hunkStart,
		hunkEnd:   hunkEnd,
		sha:       sha,
	}, nil
}
