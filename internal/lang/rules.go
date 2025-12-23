package lang

// ErrorRules contains language-specific error handling instructions
var ErrorRules = map[Language]string{
	LangGo: `GO ERROR HANDLING:
- "go.mod file not found" -> run "go mod init <module-name>" first
- "package X not found" -> run "go get <package>" to install dependency
- "undefined: X" -> check imports, might need to add import or fix typo
- "cannot find module" -> run "go mod tidy" to sync dependencies
- "build constraints exclude" -> check GOOS/GOARCH or build tags
- After fixing, always re-run "go build" to verify`,

	LangPython: `PYTHON ERROR HANDLING:
- "ModuleNotFoundError" -> run "pip install <package>" or "pip3 install <package>"
- "No module named X" -> install missing package or check virtual environment
- "SyntaxError" -> check indentation (Python uses spaces, not tabs)
- "IndentationError" -> fix inconsistent indentation
- "command not found: python" -> try "python3" instead
- Consider using "python -m venv venv" then "source venv/bin/activate" for isolation
- After fixing, re-run "python <script>.py" to verify`,

	LangNode: `NODE.JS ERROR HANDLING:
- "Cannot find module" -> run "npm install" or "npm install <package>"
- "package.json not found" -> run "npm init -y" first
- "ENOENT" -> file or directory doesn't exist, check paths
- "SyntaxError: Unexpected token" -> check for JSON errors or ES module issues
- "ERR_MODULE_NOT_FOUND" -> check import paths, might need .js extension
- "node: command not found" -> Node.js not installed or not in PATH
- Use "npm install" to install all dependencies from package.json
- After fixing, re-run "node <script>.js" or "npm start" to verify`,

	LangRust: `RUST ERROR HANDLING:
- "could not find Cargo.toml" -> run "cargo init" or "cargo new <name>" first
- "unresolved import" -> add dependency to Cargo.toml or check use statement
- "cannot find crate" -> run "cargo build" to fetch dependencies
- "borrow checker" errors -> review ownership and borrowing rules
- "mismatched types" -> check type annotations and conversions
- Run "cargo check" for faster error checking without full build
- After fixing, run "cargo build" or "cargo run" to verify`,

	LangJava: `JAVA ERROR HANDLING:
- "package does not exist" -> add dependency to pom.xml or build.gradle
- "cannot find symbol" -> check imports or class/method names
- "class X is public, should be in file named X.java" -> rename file to match class
- "javac: command not found" -> install JDK or set JAVA_HOME
- For Maven: run "mvn clean install" to build
- For Gradle: run "gradle build" or "./gradlew build"
- After fixing, re-compile with "javac *.java" or use build tool`,

	LangCSharp: `C# ERROR HANDLING:
- "could not find project file" -> run "dotnet new console" to create project
- "namespace X could not be found" -> add NuGet package with "dotnet add package X"
- "CS0246: type not found" -> check using statements and package references
- "The SDK is not installed" -> install .NET SDK
- Run "dotnet restore" to fetch dependencies
- After fixing, run "dotnet build" or "dotnet run" to verify`,

	LangCpp: `C/C++ ERROR HANDLING:
- "undefined reference" -> missing library, add -l<lib> flag to linker
- "fatal error: X.h: No such file" -> install dev package or fix include path
- "make: *** No targets" -> check Makefile exists and has correct targets
- "cmake not found" -> install cmake
- For CMake: run "cmake ." then "make"
- Check for missing -I (include) or -L (library) paths
- After fixing, re-run "make" or "g++ -o output source.cpp" to verify`,

	LangRuby: `RUBY ERROR HANDLING:
- "cannot load such file" -> run "bundle install" or "gem install <gem>"
- "Gemfile not found" -> run "bundle init" to create Gemfile
- "LoadError" -> missing gem, add to Gemfile and run "bundle install"
- "syntax error" -> check Ruby syntax, often missing "end" or typos
- Use "bundle exec" prefix to run with correct gem versions
- After fixing, re-run "ruby <script>.rb" or "bundle exec ruby <script>.rb"`,

	LangPHP: `PHP ERROR HANDLING:
- "Class 'X' not found" -> run "composer install" or "composer require <package>"
- "composer.json not found" -> run "composer init" first
- "Call to undefined function" -> enable PHP extension or install package
- "Parse error: syntax error" -> check for missing semicolons or braces
- Run "composer dump-autoload" if autoloading issues
- After fixing, re-run "php <script>.php" to verify`,

	LangSwift: `SWIFT ERROR HANDLING:
- "no such module" -> add package to Package.swift dependencies
- "cannot find type" -> check imports and type names
- "Package.swift not found" -> run "swift package init" first
- "unable to resolve dependency" -> run "swift package resolve"
- Build with "swift build", run with "swift run"
- After fixing, re-run "swift build" to verify`,

	LangKotlin: `KOTLIN ERROR HANDLING:
- "unresolved reference" -> check imports and dependencies in build.gradle.kts
- "could not find or load main class" -> check main function and package
- Use "gradle build" or "./gradlew build" to compile
- After fixing, re-run "gradle run" or "./gradlew run" to verify`,

	LangUnknown: `GENERAL ERROR HANDLING:
- Read error messages carefully - they usually indicate the problem
- Check if required tools are installed and in PATH
- Look for missing dependencies or configuration files
- Verify file paths are correct
- After fixing any error, re-run the command to verify it works`,
}

// GetErrorRules returns the error handling rules for detected languages
func GetErrorRules(langs []Language) string {
	if len(langs) == 0 {
		return ErrorRules[LangUnknown]
	}

	result := ""
	for _, lang := range langs {
		if rules, ok := ErrorRules[lang]; ok {
			if result != "" {
				result += "\n\n"
			}
			result += rules
		}
	}

	if result == "" {
		return ErrorRules[LangUnknown]
	}
	return result
}

// GetErrorRulesForLanguage returns rules for a single language
func GetErrorRulesForLanguage(lang Language) string {
	if rules, ok := ErrorRules[lang]; ok {
		return rules
	}
	return ErrorRules[LangUnknown]
}
