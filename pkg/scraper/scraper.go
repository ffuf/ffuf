package scraper

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"

	"github.com/PuerkitoBio/goquery"
)

type ScraperRule struct {
	Name         string `json:"name"`
	Rule         string `json:"rule"`
	Target       string `json:"target"`
	compiledRule *regexp.Regexp
	Type         string   `json:"type"`
	Action       []string `json:"action"`
}

type Scraper struct {
	Rules  []*ScraperRule `json:"rules"`
	active bool
}

func FromFile(path string) (ffuf.Scraper, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return &Scraper{}, err
	}
	sc := Scraper{}
	err = json.Unmarshal([]byte(data), &sc)
	for _, r := range sc.Rules {
		err = r.init()
		if err != nil {
			return &sc, err
		}
	}
	sc.active = true
	return &sc, err
}

func (s *Scraper) Active() bool {
	return s.active
}
func (s *Scraper) Execute(resp *ffuf.Response) []ffuf.ScraperResult {
	res := make([]ffuf.ScraperResult, 0)
	for _, rule := range s.Rules {
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

//init initializes the scraper rule, and returns an error in case there's an error in the syntax
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
