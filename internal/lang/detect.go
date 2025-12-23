package lang

import (
	"os"
	"path/filepath"
)

// Language represents a detected programming language
type Language string

const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangNode       Language = "node"
	LangRust       Language = "rust"
	LangJava       Language = "java"
	LangCSharp     Language = "csharp"
	LangCpp        Language = "cpp"
	LangRuby       Language = "ruby"
	LangPHP        Language = "php"
	LangSwift      Language = "swift"
	LangKotlin     Language = "kotlin"
	LangUnknown    Language = "unknown"
)

// projectFiles maps project indicator files to languages
var projectFiles = map[string]Language{
	"go.mod":           LangGo,
	"go.sum":           LangGo,
	"package.json":     LangNode,
	"package-lock.json": LangNode,
	"yarn.lock":        LangNode,
	"Cargo.toml":       LangRust,
	"Cargo.lock":       LangRust,
	"requirements.txt": LangPython,
	"pyproject.toml":   LangPython,
	"setup.py":         LangPython,
	"Pipfile":          LangPython,
	"pom.xml":          LangJava,
	"build.gradle":     LangJava,
	"build.gradle.kts": LangKotlin,
	"Gemfile":          LangRuby,
	"composer.json":    LangPHP,
	"Package.swift":    LangSwift,
	"CMakeLists.txt":   LangCpp,
	"Makefile":         LangCpp, // Could be many languages, but often C/C++
	"*.csproj":         LangCSharp,
	"*.sln":            LangCSharp,
}

// DetectLanguage detects the primary language used in the given directory
func DetectLanguage(dir string) Language {
	// Check for specific project files
	for file, lang := range projectFiles {
		// Handle glob patterns
		if file[0] == '*' {
			matches, _ := filepath.Glob(filepath.Join(dir, file))
			if len(matches) > 0 {
				return lang
			}
			continue
		}

		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return lang
		}
	}

	return LangUnknown
}

// DetectMultipleLanguages returns all detected languages in a directory
func DetectMultipleLanguages(dir string) []Language {
	seen := make(map[Language]bool)
	var langs []Language

	for file, lang := range projectFiles {
		if seen[lang] {
			continue
		}

		// Handle glob patterns
		if file[0] == '*' {
			matches, _ := filepath.Glob(filepath.Join(dir, file))
			if len(matches) > 0 {
				seen[lang] = true
				langs = append(langs, lang)
			}
			continue
		}

		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			seen[lang] = true
			langs = append(langs, lang)
		}
	}

	return langs
}
