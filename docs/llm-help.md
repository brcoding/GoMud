# LLM-Powered Help System

The GoMud help system has been enhanced with LLM (Large Language Model) capabilities to provide intelligent responses to player questions when no predefined help template exists.

## How It Works

1. When a player uses the `help` command with a query, the system first tries to match it to an existing help template.
2. If no template is found, it checks for any previously auto-generated templates that match the query.
3. If still no match is found, it calls an LLM API to generate a contextual response based on:
   - The player's specific query
   - Available commands and features in the game
   - The system prompt configured in the settings

4. If configured to do so, the system saves the LLM's response as a new template for future use, reducing API calls and ensuring consistent responses.

## Configuration

The LLM help system is configured in the `config.yaml` file under the `Integrations.LLMHelp` section:

```yaml
Integrations:
  LLMHelp:
    Enabled: true
    SystemPrompt: "You are a helpful MUD game assistant..."
    EndpointURL: "https://api.openai.com/v1/chat/completions"
    APIKey: "your-api-key-here" # Store in _datafiles/config.custom.yaml
    Model: "gpt-4.1-nano"
    TemplatePath: "templates/help"
    SaveResponses: true
```

For security reasons, it's recommended to store your API key in a separate `_datafiles/config.custom.yaml` file that's added to `.gitignore`:

```yaml
Integrations:
  LLMHelp:
    APIKey: "your-api-key-here"
```

### Configuration Options

- **Enabled**: Whether the LLM help system is enabled
- **SystemPrompt**: The system prompt to use for LLM requests
- **EndpointURL**: The URL for the LLM API
- **APIKey**: API key (if needed)
- **Model**: The model to use for LLM requests
- **TemplatePath**: Path to store generated templates
- **SaveResponses**: Whether to save LLM responses as templates

## Supported LLM Services

The system is designed to work with any OpenAI-compatible API endpoint. This includes:

1. **OpenAI API**: Use the official OpenAI API with models like:
   - GPT-4.1-nano (recommended for help responses)
   - GPT-3.5-turbo
   - GPT-4o

2. **Local LLMs**: Run models locally using tools like:
   - [Ollama](https://ollama.ai)
   - [LocalAI](https://github.com/localai/localai)
   - [LM Studio](https://lmstudio.ai)

3. **Self-hosted API services**: Set up your own API server

## Template Management

Auto-generated templates are stored in the `templates/help/auto/` directory with filenames derived from the player's query. These files are in Markdown format and can be manually edited if needed.

## Best Practices

1. **System Prompt**: Craft a clear system prompt that instructs the LLM to provide accurate, concise game information.
2. **Use Local Models**: For offline use or reduced latency, consider running local models.
3. **Review Generated Templates**: Periodically review auto-generated templates for quality and accuracy.
4. **Create Custom Templates**: For common queries, create custom templates rather than relying on the LLM.

## Troubleshooting

- **API Connection Issues**: Verify the `EndpointURL` is correct and the API service is running.
- **Poor Quality Responses**: Refine the system prompt to provide better guidance to the LLM.
- **High Latency**: Consider using a local LLM service or caching frequently used responses.

For issues with the help system, check the server logs for entries with `llm-help` as the component name. 