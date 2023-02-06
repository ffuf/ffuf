package scraper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"

	"github.com/PuerkitoBio/goquery"
)

type ScraperRule struct {
	Name         string `json:"name"`
	Rule         string `json:"rule"`
	Target       string `json:"target"`
	compiledRule *regexp.Regexp
	Type         string   `json:"type"`
	OnlyMatched  bool     `json:"onlymatched"`
	Action       []string `json:"action"`
}

type ScraperGroup struct {
	Rules  []*ScraperRule `json:"rules"`
	Name   string         `json:"groupname"`
	Active bool           `json:"active"`
}

type Scraper struct {
	Rules []*ScraperRule
}

func readGroupFromFile(filename string) (ScraperGroup, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return ScraperGroup{Rules: make([]*ScraperRule, 0)}, err
	}
	sc := ScraperGroup{}
	err = json.Unmarshal([]byte(data), &sc)
	return sc, err
}

func FromDir(dirname string, activestr string) (ffuf.Scraper, ffuf.Multierror) {
	scr := Scraper{Rules: make([]*ScraperRule, 0)}
	errs := ffuf.NewMultierror()
	activegrps := parseActiveGroups(activestr)
	all_files, err := os.ReadDir(ffuf.SCRAPERDIR)
	if err != nil {
		errs.Add(err)
		return &scr, errs
	}
	for _, filename := range all_files {
		if filename.Type().IsRegular() && strings.HasSuffix(filename.Name(), ".json") {
			sg, err := readGroupFromFile(filepath.Join(dirname, filename.Name()))
			if err != nil {
				cerr := fmt.Errorf("%s : %s", filepath.Join(dirname, filename.Name()), err)
				errs.Add(cerr)
				continue
			}
			if (sg.Active && isActive("all", activegrps)) || isActive(sg.Name, activegrps) {
				for _, r := range sg.Rules {
					err = r.init()
					if err != nil {
						cerr := fmt.Errorf("%s : %s", filepath.Join(dirname, filename.Name()), err)
						errs.Add(cerr)
						continue
					}
					scr.Rules = append(scr.Rules, r)
				}
			}
		}
	}
	return &scr, errs
}

// FromFile initializes a scraper instance and reads rules from a file
func (s *Scraper) AppendFromFile(path string) error {
	sg, err := readGroupFromFile(path)
	if err != nil {
		return err
	}

	for _, r := range sg.Rules {
		err = r.init()
		if err != nil {
			continue
		}
		s.Rules = append(s.Rules, r)
	}

	return err
}

func (s *Scraper) Execute(resp *ffuf.Response, matched bool) []ffuf.ScraperResult {
	res := make([]ffuf.ScraperResult, 0)
	for _, rule := range s.Rules {
		if !matched && rule.OnlyMatched {
			// pass this rule as there was no match
			continue
		}
		sourceData := ""
		if rule.Target == "body" {
			sourceData = string(resp.Data)
		} else if rule.Target == "headers" {
			sourceData = headerString(resp.Headers)
		} else {
			sourceData = headerString(resp.Headers) + string(resp.Data)
		}
		val := rule.Check(sourceData)
		if len(val) > 0 {
			res = append(res, ffuf.ScraperResult{
				Name:    rule.Name,
				Type:    rule.Type,
				Action:  rule.Action,
				Results: val,
			})
		}
	}
	return res
}

// init initializes the scraper rule, and returns an error in case there's an error in the syntax
func (r *ScraperRule) init() error {
	var err error
	if r.Type == "regexp" {
		r.compiledRule, err = regexp.Compile(r.Rule)
		if err != nil {
			return err
		}
	}
	return err
}

func (r *ScraperRule) Check(data string) []string {
	if r.Type == "regexp" {
		return r.checkRegexp(data)
	} else if r.Type == "query" {
		return r.checkQuery(data)
	}
	return []string{}
}

func (r *ScraperRule) checkQuery(data string) []string {
	val := make([]string, 0)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data))
	if err != nil {
		return []string{}
	}
	doc.Find(r.Rule).Each(func(i int, sel *goquery.Selection) {
		val = append(val, sel.Text())
	})
	return val
}

func (r *ScraperRule) checkRegexp(data string) []string {
	val := make([]string, 0)
	if r.compiledRule != nil {
		res := r.compiledRule.FindAllStringSubmatch(data, -1)
		for _, grp := range res {
			val = append(val, grp...)
		}
		return val
	}
	return []string{}
}
