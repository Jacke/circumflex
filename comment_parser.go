package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	term "github.com/MichaelMure/go-term-text"
	"github.com/eidolon/wordwrap"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

const (
	NORMAL        = "\033[0m"
	BOLD          = "\033[1m"
	DIMMED        = "\033[2m"
	ITALIC        = "\033[3m"
	GREEN         = "\033[32;m"
	RED           = "\033[31;m"
	Link_1        = "\033]8;;"
	Link_2        = "\a"
	Link_3        = "\033]8;;\a"
	NewLine       = "\n"
	DoubleNewLine = "\n\n"
)

type Comments struct {
	Author        string      `json:"user"`
	Title         string      `json:"title"`
	Comment       string      `json:"content"`
	CommentsCount int         `json:"comments_count"`
	Time          string      `json:"time_ago"`
	Points        int         `json:"points"`
	URL           string      `json:"url"`
	Domain        string      `json:"domain"`
	Replies       []*Comments `json:"comments"`
}

func appendCommentsHeader(c Comments, commentTree *string) {
	headline := BOLD + c.Title + NORMAL + DIMMED + "  (" + c.Domain + ")" + NORMAL + NewLine
	infoLine := strconv.Itoa(c.Points) + " points by " + BOLD + c.Author + NORMAL + " " + c.Time + " | " + strconv.Itoa(c.CommentsCount) + " comments" + DoubleNewLine
	*commentTree += headline + infoLine
	titleBarLength := term.Len(headline)

	fullComment := ""
	comment := parseComment(c.Comment)
	wrapper := wordwrap.Wrapper(titleBarLength, false)

	commentLines := strings.Split(comment, NewLine)
	lastParagraph := len(commentLines) - 1
	for i, line := range commentLines {
		wrapped := wrapper(line)
		wrappedAndIndentedComment := wordwrap.Indent(wrapped, getIndentBlock(0), true)
		if i == lastParagraph {
			fullComment += wrappedAndIndentedComment + NewLine
		} else {
			fullComment += wrappedAndIndentedComment + DoubleNewLine
		}
	}

	*commentTree += fullComment

	for i := 0; i < titleBarLength; i++ {
		*commentTree += "-"
	}

	*commentTree += DoubleNewLine

}

func getDomainText(domain string, URL string, id string) string {
	if domain != "" {
		return DIMMED + "  (" + getHyperlinkText(URL, domain) + ")" + NORMAL
	} else {
		linkToComments := "https://news.ycombinator.com/item?id=" + id
		linkText := "item?id=" + id
		return DIMMED + "  (" + getHyperlinkText(linkToComments, linkText) + ")" + NORMAL
	}
}

func getHyperlinkText(URL string, text string) string {
	return fmt.Sprintf("%d%d%d%d%d", Link_1, URL, Link_2, text, Link_3)
}

func prettyPrintComments(c Comments, commentTree *string, indentlevel int, op string) string {
	x, _ := terminal.Width()
	rightPadding := 3
	comment := parseComment(c.Comment)
	wrapper := wordwrap.Wrapper(int(x)-indentlevel-rightPadding, false)
	markedAuthor := markOPAndMods(c.Author, op)

	fullComment := ""
	commentLines := strings.Split(comment, NewLine)
	for _, line := range commentLines {
		wrapped := wrapper(line)
		wrappedAndIndentedComment := wordwrap.Indent(wrapped, getIndentBlock(indentlevel), true)
		fullComment += wrappedAndIndentedComment + DoubleNewLine
	}

	wrappedAndIndentedAuthor := wordwrap.Indent(markedAuthor, getIndentBlock(indentlevel), true)
	wrappedAndIndentedComment := BOLD + wrappedAndIndentedAuthor + NORMAL + " " + getRightAlignedTimeAgo(markedAuthor, c.Time, indentlevel)
	wrappedAndIndentedComment += fullComment

	*commentTree = *commentTree + wrappedAndIndentedComment
	for _, s := range c.Replies {
		prettyPrintComments(*s, commentTree, indentlevel+5, op)
	}
	return *commentTree
}

func getRightAlignedTimeAgo(author string, timeAgo string, indentLevel int) string {
	screenWidth, _ := terminal.Width()
	authorLength := term.Len(author)
	timeAgoLength := term.Len(timeAgo)
	paddingBetweenAuthorAndTime := ""
	padding := 6

	numberOfSpaces := int(screenWidth) - authorLength - timeAgoLength - padding - indentLevel

	for i := 0; i < numberOfSpaces; i++ {
		paddingBetweenAuthorAndTime += " "
	}

	return paddingBetweenAuthorAndTime + DIMMED + timeAgo + NORMAL + NewLine

}

func markOPAndMods(author, op string) string {
	markedAuthor := author
	if author == "dang" || author == "sctb" {
		markedAuthor = author + GREEN + " mod" + NORMAL
	}
	if author == op {
		markedAuthor = markedAuthor + RED + " OP" + NORMAL
	}
	return markedAuthor
}

func getIndentBlock(level int) string {
	indentation := ""
	for i := 0; i < level; i++ {
		indentation = indentation + " "
	}
	return indentation
}

func parseComment(comment string) string {
	fixedHTML := replaceHTML(comment)
	fixedHTMLAndCharacters := replaceCharacters(fixedHTML)
	fixedHTMLAndCharactersAndHrefs := handleHrefTag(fixedHTMLAndCharacters)
	return fixedHTMLAndCharactersAndHrefs
}

func replaceCharacters(input string) string {
	input = strings.ReplaceAll(input, "&#x27;", "'")
	input = strings.ReplaceAll(input, "&gt;", ">")
	input = strings.ReplaceAll(input, "&lt;", "<")
	input = strings.ReplaceAll(input, "&#x2F;", "/")
	input = strings.ReplaceAll(input, "&quot;", "\"")
	input = strings.ReplaceAll(input, "&amp;", "&")
	return input
}

func replaceHTML(input string) string {
	input = strings.Replace(input, "<p>", "", 1)

	input = strings.ReplaceAll(input, "<p>", NewLine)
	input = strings.ReplaceAll(input, "<i>", ITALIC)
	input = strings.ReplaceAll(input, "</i>", NORMAL)
	input = strings.ReplaceAll(input, "<pre><code>", DIMMED)
	input = strings.ReplaceAll(input, "</code></pre>", NORMAL)
	return input
}

func handleHrefTag(input string) string {
	var expForFirstTag = regexp.MustCompile(`<a href="`)
	replacedInput := expForFirstTag.ReplaceAllString(input, Link_1)

	var expForSecondTag = regexp.MustCompile(`" rel="nofollow">`)
	replacedInput = expForSecondTag.ReplaceAllString(replacedInput, Link_2)

	var expForThirdTag = regexp.MustCompile(`<\/a>`)
	replacedInput = expForThirdTag.ReplaceAllString(replacedInput, Link_3)

	return replacedInput
}
