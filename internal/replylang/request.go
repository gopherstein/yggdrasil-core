package replylang

import (
	"regexp"
	"strings"
)

// Language names as people write them, in English and in the languages of
// the app (i18n/languages.json), each with its tag.
var names = map[string]string{
	// English
	"english": "en", "german": "de", "spanish": "es", "french": "fr", "italian": "it", "portuguese": "pt",
	"brazilian portuguese": "pt-BR", "japanese": "ja", "korean": "ko", "chinese": "zh-Hans",
	"simplified chinese": "zh-Hans", "traditional chinese": "zh-Hant", "dutch": "nl", "russian": "ru",
	"arabic": "ar", "hebrew": "he", "hindi": "hi", "polish": "pl", "turkish": "tr", "swedish": "sv",
	"ukrainian": "uk", "vietnamese": "vi", "indonesian": "id", "thai": "th", "greek": "el",
	// German
	"englisch": "en", "deutsch": "de", "spanisch": "es", "französisch": "fr", "italienisch": "it",
	"portugiesisch": "pt", "japanisch": "ja", "koreanisch": "ko", "chinesisch": "zh-Hans",
	// Spanish
	"inglés": "en", "ingles": "en", "alemán": "de", "aleman": "de", "español": "es", "francés": "fr",
	"frances": "fr", "italiano": "it", "portugués": "pt", "japonés": "ja", "coreano": "ko", "chino": "zh-Hans",
	// French
	"anglais": "en", "allemand": "de", "espagnol": "es", "français": "fr", "francais": "fr", "italien": "it",
	"portugais": "pt", "japonais": "ja", "coréen": "ko", "chinois": "zh-Hans",
	// Italian
	"inglese": "en", "tedesco": "de", "spagnolo": "es", "francese": "fr", "portoghese": "pt",
	"giapponese": "ja", "cinese": "zh-Hans",
	// Portuguese
	"inglês": "en", "alemão": "de", "espanhol": "es", "francês": "fr", "português": "pt", "japonês": "ja",
	// Japanese, Korean, Chinese
	"英語": "en", "日本語": "ja", "韓国語": "ko", "中国語": "zh-Hans", "ドイツ語": "de", "フランス語": "fr", "スペイン語": "es",
	"영어": "en", "한국어": "ko", "일본어": "ja", "중국어": "zh-Hans", "독일어": "de", "프랑스어": "fr", "스페인어": "es",
	"英文": "en", "英语": "en", "中文": "zh-Hans", "简体中文": "zh-Hans", "繁体中文": "zh-Hant",
	"繁體中文": "zh-Hant", "日语": "ja", "日文": "ja", "韩语": "ko", "韩文": "ko", "德语": "de", "法语": "fr", "西班牙语": "es",
}

// requestPatterns find a language asked for. Each must say to answer or
// write in it, so a question such as "how do you say this in English?" is
// not one: the person still wants the answer in their own language.
var requestPatterns = func() []*regexp.Regexp {
	alt := make([]string, 0, len(names))
	for name := range names {
		alt = append(alt, regexp.QuoteMeta(name))
	}
	// Longer names first, so "simplified chinese" wins over "chinese".
	sortByLength(alt)
	lang := `(` + strings.Join(alt, "|") + `)`
	return []*regexp.Regexp{
		// English: answer in German / reply in German / in German, please / German please
		regexp.MustCompile(`(?i)\b(?:answer|reply|respond|write|speak|talk|explain|continue|translate (?:it|this|that|your answer)? ?into)\b[^.?!\n]{0,40}?\b(?:in|into) ` + lang + `\b`),
		regexp.MustCompile(`(?i)(?:^|[.!?]\s*)in ` + lang + `,? please\b`),
		regexp.MustCompile(`(?i)\b` + lang + ` please[.!]?\s*$`),
		// German: antworte auf Englisch / bitte auf Deutsch
		regexp.MustCompile(`(?i)\b(?:antworte|antworten|antwort|schreib|schreibe|schreiben|erkläre|erklär|sprich|rede|übersetze)\b[^.?!\n]{0,40}?\bauf ` + lang + `\b`),
		regexp.MustCompile(`(?i)\bbitte auf ` + lang + `\b|\bauf ` + lang + `,? bitte\b`),
		// Spanish, French, Italian, Portuguese: responde en inglés / réponds en anglais / rispondi in inglese / responda em inglês
		regexp.MustCompile(`(?i)\b(?:responde|respóndeme|respondeme|contesta|contéstame|escribe|explica|háblame|habla|réponds|répondez|répondre|écris|écrivez|explique|expliquez|parle|parlez|rispondi|risponda|rispondere|scrivi|scriva|spiega|parla|responda|responder|escreva|explique|fale)\b[^.?!\n]{0,40}?\b(?:en|in|em) ` + lang + `\b`),
		regexp.MustCompile(`(?i)\b(?:en|in|em) ` + lang + `,? (?:por favor|s'il (?:te|vous) plaît|per favore)\b`),
		// Japanese: 英語で答えて / 英語で返事
		regexp.MustCompile(lang + `で(?:答え|こたえ|返事|返信|回答|説明|書い|話し|お願い)`),
		// Korean: 영어로 답해 / 영어로 대답
		regexp.MustCompile(lang + `로\s?(?:답|대답|말|설명|써|작성|부탁)`),
		// Chinese: 用英文回答 / 请用中文回复
		regexp.MustCompile(`用` + lang + `(?:回答|回复|回覆|說|说|写|寫|解释|解釋)`),
	}
}()

func sortByLength(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && len(s[j]) > len(s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Requested is the language a message asks the answer to be in, such as
// "en" for "Answer this one in English.", if it asks for one.
func Requested(text string) (tag string, ok bool) {
	text = strings.TrimSpace(text)
	best := -1
	for _, re := range requestPatterns {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			// The last request in a message wins: "in German… actually, in English".
			if m[0] > best && m[2] >= 0 {
				if t, found := names[strings.ToLower(text[m[2]:m[3]])]; found {
					best, tag = m[0], t
				}
			}
		}
	}
	return tag, best >= 0
}
