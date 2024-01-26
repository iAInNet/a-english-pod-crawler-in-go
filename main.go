package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gocolly/colly/v2"
)

type EnglishPodContent struct {
	Title                   string           // 标题
	SerialNo                string           // 序号
	AudioClip               string           // 音频文件链接
	Dialogue                []string         // 对话
	KeyVocabulary           []VocabularyItem // 关键词汇
	SupplementaryVocabulary []VocabularyItem // 补充词汇
}

type VocabularyItem struct {
	Vocabulary   string // 单词
	PartOfSpeech string // 词性
	Meaning      string // 词义
}

const tabPadding = 3
const markdownFileSavePath = "/tmp"

func main() {
	// crawl englishpods archive website
	// and save as markdown file
	englishPodCrawler()
}

func englishPodCrawler() {
	c := colly.NewCollector(
		colly.AllowedDomains(),
		colly.MaxDepth(2),
	)
	c.SetRequestTimeout(20 * time.Second)

	contentCollector := c.Clone()
	contentCollector.Async = true
	contentCollector.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 3})

	// 遍历 HTML 文本 a 链接
	c.OnHTML("a.stealth.download-pill[href*=html]", func(e *colly.HTMLElement) {
		contentCollector.Visit(e.Request.AbsoluteURL(e.Attr("href")))
	})

	// 遍历 audio clip a 链接
	// c.OnHTML("a.stealth.download-pill[href*=dg\\.mp3]", func(h *colly.HTMLElement) {
	// 	c.Visit(h.Request.AbsoluteURL(h.Attr("href")))
	// })

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("visit error: ", err)
	})

	// 内容页面请求
	contentCollector.OnRequest(func(r *colly.Request) {
		fmt.Println("content visit url: ", r.URL.String())
	})

	// 错误处理
	contentCollector.OnError(func(r *colly.Response, err error) {
		fmt.Println("content visit error: ", err)
	})

	// 解析页面，组装内容
	contentCollector.OnHTML("body", func(e *colly.HTMLElement) {
		englishpodContent := EnglishPodContent{}
		englishpodContent.Title = e.ChildText("body > h1 > a")
		serialNoRawString := e.ChildText("body > h1 > span")
		serialNoRegex := regexp.MustCompile(`\(.*(\d{4})\)`)
		serialNoMatchArr := serialNoRegex.FindStringSubmatch(serialNoRawString)
		if len(serialNoMatchArr) >= 2 {
			englishpodContent.SerialNo = serialNoMatchArr[1]
			englishpodContent.AudioClip = fmt.Sprintf("https://archive.org/download/englishpod_all/englishpod_%sdg.mp3", englishpodContent.SerialNo)
		}
		e.ForEach("table", func(inx int, table *colly.HTMLElement) {
			// dialogue
			if inx == 0 {
				dialogue := make([]string, 0)
				table.ForEach("tbody > tr", func(_ int, tr *colly.HTMLElement) {
					speach := tr.ChildText("td")
					if len(speach) > 0 {
						dialogue = append(dialogue, speach)
					}
				})
				englishpodContent.Dialogue = dialogue
			}
			// key vocabulary
			if inx == 1 {
				keyVocabulary := make([]VocabularyItem, 0)
				table.ForEach("tbody > tr", func(_ int, tr *colly.HTMLElement) {
					tmp := VocabularyItem{}
					tmp.Vocabulary = tr.DOM.Children().Eq(0).Text()
					tmp.PartOfSpeech = tr.DOM.Children().Eq(1).Text()
					tmp.Meaning = tr.DOM.Children().Eq(2).Text()
					keyVocabulary = append(keyVocabulary, tmp)
				})
				englishpodContent.KeyVocabulary = keyVocabulary
			}
			// supplementary vocabulary
			if inx == 2 {
				supplementaryVocabulary := make([]VocabularyItem, 0)
				table.ForEach("tbody > tr", func(_ int, tr *colly.HTMLElement) {
					tmp := VocabularyItem{}
					tmp.Vocabulary = tr.DOM.Children().Eq(0).Text()
					tmp.PartOfSpeech = tr.DOM.Children().Eq(1).Text()
					tmp.Meaning = tr.DOM.Children().Eq(2).Text()
					supplementaryVocabulary = append(supplementaryVocabulary, tmp)
				})
				englishpodContent.SupplementaryVocabulary = supplementaryVocabulary
			}
		})
		be, err := json.Marshal(englishpodContent)
		if err != nil {
			log.Fatal("Invalid Content Structure!")
			return
		}
		fmt.Println(string(be))
		saveAsMarkdownFile(englishpodContent)
	})

	// start point
	fmt.Println("crawling...")
	c.Visit("https://archive.org/details/englishpod_all")

	// Wait until threads are finished
	contentCollector.Wait()
}

func saveAsMarkdownFile(englishPodContent EnglishPodContent) {
	mdFileName := fmt.Sprintf("%s/English Pod %s %s.md", markdownFileSavePath, englishPodContent.SerialNo, englishPodContent.Title)
	mdFileHandle, err := os.Create(mdFileName)
	if err != nil {
		log.Fatalf("Cannot create markdown file %q: %s\n", mdFileName, err)
		return
	}
	defer mdFileHandle.Close()

	// dialogue
	dialogue := strings.Join(englishPodContent.Dialogue, "\n\n")

	// key vocabulary
	keyVocabulary := tabWriterPaddingVocabulary(englishPodContent.KeyVocabulary)

	// supplementary vocabulary
	supVocabulary := tabWriterPaddingVocabulary(englishPodContent.SupplementaryVocabulary)

	// construct content
	content := ""
	content += "## Audio Clip\n"
	content += fmt.Sprintf("[Audio Clip %s](%s)", englishPodContent.SerialNo, englishPodContent.AudioClip)
	content += "\n\n"
	content += "## Dialogue\n"
	content += dialogue
	content += "\n\n"
	content += "## Key Vocabulary\n"
	content += keyVocabulary
	content += "\n"
	content += "## Supplementary Vocabulary\n"
	content += supVocabulary

	// write to file
	mdFileHandle.WriteString(content)
	fmt.Printf("generate markdown file, %s\n", mdFileName)
}

func tabWriterPaddingVocabulary(vocabularyList []VocabularyItem) string {
	if len(vocabularyList) <= 0 {
		return ""
	}

	var (
		contentBuffer bytes.Buffer
		w             = tabwriter.NewWriter(&contentBuffer, 0, 0, tabPadding, ' ', 0)
	)
	for j := 0; j < len(vocabularyList); j++ {
		_, err := fmt.Fprintf(w, "**%s**\t%s\t%s\n", vocabularyList[j].Vocabulary, vocabularyList[j].PartOfSpeech, vocabularyList[j].Meaning)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}
	w.Flush()
	return contentBuffer.String()
}
