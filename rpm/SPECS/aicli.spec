Name:           aicli
Version:        0.6.1
Release:        1%{?dist}
Summary:        Command-line AI coding assistant

License:        MIT
URL:            https://github.com/glennswest/aicli

%description
A command-line AI coding assistant with tool execution capabilities.
Works with any OpenAI-compatible API.

Features:
- Interactive chat with AI models
- Tool execution: shell commands, file operations, git, web search
- Session recording and playback
- Auto-versioning on commits (semver)
- Piped input support for scripting

%install
mkdir -p %{buildroot}/usr/local/bin
cp %{_sourcedir}/aicli %{buildroot}/usr/local/bin/aicli
chmod 755 %{buildroot}/usr/local/bin/aicli

%files
/usr/local/bin/aicli

%changelog
* Sun Dec 08 2024 aicli maintainer
- Initial release 0.1.0
