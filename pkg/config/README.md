# The Config Package

ffuf's configuration has become complex, since it takes configurations from multiple configuration sources like the commandline, multiple configuration files and HTTP request files (sometimes called HTTP templates). Inputs from these configuration sources have to be merged into one coherent configuration and it's consistency validated.

The config package aims to simplify and formalize the configuration of ffuf, while making it flexible and extensible. It provides:
  * One simple function call to obtain a configuration from a standard hierarchy of all configurations sources
  * A mechanism to merge the defined configuration sources
  * A mechanism to validate the merged configuration and supply error messages to the caller
  * A way to add a configuration source to ffuf
  * A way to add validation to new or changed options without knowing the code base

# Concepts

## Hierarchy of Configuration Sources

When configuration options are supplied by many sources, they must be merged coherently. That mostly means defining a way which options gets chosen when several configuration sources set different values for the same option, thus defining a hierarchy. Without modification, this package defines a **standard hierarchy** as follows in ascending order:
  * configuration files from default locations inside the user's home directory (least dominant):
    * a `.ffufrc` file found in the current working directory
    * if the above does not exist, a `.fuffrc` file inside the user configuration directory (Linux: `XDG_CONFIG_HOME` or `$HOME/.config`, Windows: `%AppData`, macOS: `$HOME/Library/Application Support`)
    * if the above does not exist, a `.ffufrc` file inside the user home directory (*nix: `$HOME`, Windows: `%USERPROFILE%`)
  * an HTTP request template, pointed to by the `-request` flag
  * an explicit configuration file, pointed to by `-config`
  * the commandline options (most dominant)

These options, merged and validated, are returned by a single call to `config.Get()`. If a different hierarchy with a subset of the sources is desired, these can be defined via `config.SetSources()` which takes an arbitrary amount of configuration sources (see [api.go](api.go)).

## Configuration Sources

A configuration source, as defined by the `ConfigSource` type, is a [function closure](https://en.wikipedia.org/wiki/Closure_(computer_programming)) which carries in it's environment all parameters to obtain it's configuration options, so that this function can be called without any arguments. Functions like `SourceFromCmdline` or `SourceFromFile` produce these closures. See [provide.go](provide.go) and [provide_test.go](provide_test.go) under the EXAMPLES section for more details.

## Translators
A translator, as defined by the `Translator` type, is a functions which takes a reference to a `ConfigOptions` struct (representing the supplied options of a source) and a reference to a `Config` struct (representing the final, validated configuration), picks one aspect of the options, translates it to a configuration, thereby either validating it or returning an error in case of illegal values or an incompatibility with other options. A Translator does not terminate the program but rather return a descriptive error message. Further, a Translator is self-contained and thus does not depend on the prior execution of other Translators for it's operation. See [translate.go](translate.go) and [translate_test.go](translate_test.go) under the EXAMPLES section for more details.

# Extending the Configuration
To introduce a feature, often a new configuration option has to be defined and validated. The config package aims to simplify the process. To introduce configuration for the feature, these steps should be followed:
  1. Add the options to the `ConfigOptions` and `Config` structs inside [config_options.go](config_options.go).
  2. Add a default value to `NewConfigOptions()` and `NewConfig()` inside [config_options.go](config_options.go), **especially** if the type of the options is a reference type like a pointer, slice or map, because their zero value is `nil`, which makes the `merge()` function panic.
  3. Define a Translator inside [translate.go](translate.go) under the DEFAULT TRANSLATORS section.
  4. Add the Translator to the `_translators` slice inside [api.go](api.go). Order should not matter

Now a call to `config.Get()` will consider your new option.

# Caveats
It is difficult to distinguish unset configuration options from the default options (especially for boolean flags, which have only two states), an option is considered unset if it is the default value. Thus setting a default value even in the most dominant configuration source will be considered as unset and overwritten by a value set in a less dominant configuration source, running contrary to the hierarchy. So it it is desired to leave an option as it's default value, it should be unset from all options.
