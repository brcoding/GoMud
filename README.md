# GoMud

![image](feature-screenshots/splash.png)

GoMud is an in-development open source MUD (Multi-user Dungeon) game world and library.

It ships with a default world to play in, but can be overwritten or modified to build your own world using built-in tools.

# User Support

If you have comments, questions, suggestions:

[Github Discussions](https://github.com/GoMudEngine/GoMud/discussions) - Don't be shy. Your questions or requests might help others too.

[Discord Server](https://discord.gg/cjukKvQWyy) - Get more interactive help in the GoMud Discord server.

[Guides](_datafiles/guides/README.md) - Community created guides to help get started.

# Contributor Guide

Interested in contributing? Check out our [CONTRIBUTING.md](https://github.com/GoMudEngine/GoMud/blob/master/.github/CONTRIBUTING.md) to learn about the process.

## Screenshots

Click below to see in-game screenshots of just a handful of features:

[![Feature Screenshots](feature-screenshots/screenshots-thumb.png "Feature Screenshots")](feature-screenshots/README.md)

## ANSI Colors

Colorization is handled through extensive use of my [github.com/GoMudEngine/ansitags](https://github.com/GoMudEngine/ansitags) library.

## Small Feature Demos

- [Auto-complete input](https://youtu.be/7sG-FFHdhtI)
- [In-game maps](https://youtu.be/navCCH-mz_8)
- [Quests / Quest Progress](https://youtu.be/3zIClk3ewTU)
- [Lockpicking](https://youtu.be/-zgw99oI0XY)
- [Hired Mercs](https://youtu.be/semi97yokZE)
- [TinyMap](https://www.youtube.com/watch?v=VLNF5oM4pWw) (okay not much of a "feature")
- [256 Color/xterm](https://www.youtube.com/watch?v=gGSrLwdVZZQ)
- [Customizable Prompts](https://www.youtube.com/watch?v=MFkmjSTL0Ds)
- [Mob/NPC Scripting](https://www.youtube.com/watch?v=li2k1N4p74o)
- [Room Scripting](https://www.youtube.com/watch?v=n1qNUjhyOqg)
- [Kill Stats](https://www.youtube.com/watch?v=4aXs8JNj5Cc)
- [Searchable Inventory](https://www.youtube.com/watch?v=iDUbdeR2BUg)
- [Day/Night Cycles](https://www.youtube.com/watch?v=CiEbOp244cw)
- [Web Socket "Virtual Terminal"](https://www.youtube.com/watch?v=L-qtybXO4aw)
- [Alternate Characters](https://www.youtube.com/watch?v=VERF2l70W34)
- [LLM-Powered Help System](docs/llm-help.md) - AI-enhanced help for players

## Connecting

_TELNET_ : connect to `localhost` on port `33333` with a telnet client

_WEB CLIENT_: [http://localhost/webclient](http://localhost/webclient)

**Default Username:** _admin_

**Default Password:** _password_

## Env Vars

When running several environment variables can be set to alter behaviors of the mud:

- **CONFIG_PATH**_=/path/to/alternative/config.yaml_ - This can provide a path to a copy of the config.yaml containing only values you wish to override. This way you don't have to modify the original config.yaml
- **LOG_PATH**_=/path/to/log.txt_ - This will write all logs to a specified file. If unspecified, will write to _stderr_.
- **LOG_LEVEL**_={LOW/MEDIUM/HIGH}_ - This sets how verbose you want the logs to be. _(Note: Log files rotate every 100MB)_
- **LOG_NOCOLOR**_=1_ - If set, logs will be written without colorization.

# Why Go?

Why not?

Go provides a lot of terrific benefits such as:

- Compatible - High degree of compatibility across platforms or CPU Architectures. Go code quite painlessly compiles for Windows, Linux, ARM, etc. with minimal to no changes to the code.
- Fast - Go is fast. From execution to builds. The current GoMud project builds on a Macbook in less than a couple of seconds.
- Opinionated - Go style and patterns are well established and provide a reliable way to dive into a project and immediately feel familiar with the style.
- Modern - Go is a relatively new/modern language without the burden of "every feature people thought would be useful in the last 30 or 40 years" added to it.
- Upgradable - Go's promise of maintaining backward compatibility means upgrading versions over time remains a simple and painless process (If not downright invisible).
- Statically Linked - If you have the binary, you have the working program. Externally linked dependencies (and whether you have them) are not an issue.
- No Central Registries - Go is built to naturally incorporate library includes straight from their repos (such as git). This is neato.
- Concurrent - Go has concurrency built in as a feature of the language, not a library you include.

## Custom Configuration

To use the OpenAI API for LLM-powered help instead of local models:

1. Create a file named `_datafiles/config.custom.yaml` with your API configuration:

```yaml
Integrations:
  LLM:
    Enabled: true
    Provider: "openai"
    Model: "gpt-4.1-nano"
    BaseURL: "https://api.openai.com/v1"
    Temperature: 0.7
  
  LLMHelp:
    Enabled: true
    EndpointURL: "https://api.openai.com/v1/chat/completions"
    APIKey: "your-api-key-here"
    Model: "gpt-4.1-nano"
    SaveResponses: true
```

2. Replace `your-api-key-here` with your actual OpenAI API key.

3. Set the CONFIG_PATH environment variable to point to your custom config:

```bash
# On Linux/Mac:
export CONFIG_PATH=_datafiles/config.custom.yaml

# On Windows PowerShell:
$env:CONFIG_PATH="_datafiles/config.custom.yaml"

# On Windows Command Prompt:
set CONFIG_PATH=_datafiles/config.custom.yaml
```

4. Start the server with the environment variable set:

```bash
# Run directly with the environment variable set
CONFIG_PATH=_datafiles/config.custom.yaml go run .

# Or use the variable you exported earlier
go run .
```

5. This file is automatically added to `.gitignore` to prevent committing your API key to the repository.

For more details on LLM configuration, see [docs/llm-help.md](docs/llm-help.md).
