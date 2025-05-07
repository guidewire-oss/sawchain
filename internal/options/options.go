package options

import (
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/util"
)

const (
	errNil              = "options is nil"
	errRequired         = "required argument(s) not provided"
	errObjectAndObjects = "client.Object and []client.Object arguments both provided"
)

// Options is a common struct for options used in Sawchain operations.
type Options struct {
	Timeout  time.Duration   // Timeout for eventual assertions.
	Interval time.Duration   // Polling interval for eventual assertions.
	Template string          // Template content for Chainsaw resource operations.
	Bindings map[string]any  // Template bindings for Chainsaw resource operations.
	Object   client.Object   // Object to store state for single-resource operations.
	Objects  []client.Object // Slice to store state for multi-resource operations.
}

// ProcessTemplate extracts content from the given template string or file and sanitizes it
// by de-indenting non-empty lines and pruning empty documents.
func ProcessTemplate(template string) (string, error) {
	// Extract content
	var content string
	if util.IsExistingFile(template) {
		var err error
		content, err = util.ReadFileContent(template)
		if err != nil {
			return "", fmt.Errorf("failed to read template file: %v", err)
		}
	} else {
		content = template
	}
	// Sanitize content
	sanitized, err := util.PruneYAML(util.DeindentYAML(content))
	if err != nil {
		return "", fmt.Errorf("failed to sanitize template content: %v", err)
	}
	return sanitized, nil
}

// parse parses variable arguments into an Options struct.
//   - If includeDurations is true, checks for Timeout and Interval; otherwise disallows them.
//   - If includeObject is true, checks for Object; otherwise disallows it.
//   - If includeObjects is true, checks for Objects; otherwise disallows it.
//   - If includeTemplate is true, checks for Template; otherwise disallows it.
func parse(
	includeDurations bool,
	includeObject bool,
	includeObjects bool,
	includeTemplate bool,
	args ...interface{},
) (*Options, error) {
	opts := &Options{
		Bindings: map[string]any{},
	}

	for _, arg := range args {
		if includeDurations {
			// Check for Timeout and Interval
			if d, ok := util.AsDuration(arg); ok {
				if opts.Timeout != 0 && opts.Interval != 0 {
					return nil, errors.New("too many duration arguments provided")
				} else if opts.Timeout == 0 {
					opts.Timeout = d
				} else if d > opts.Timeout {
					return nil, errors.New("provided interval is greater than timeout")
				} else {
					opts.Interval = d
				}
				continue
			}
		}

		if includeObject {
			// Check for Object
			if obj, ok := util.AsObject(arg); ok {
				if opts.Object != nil {
					return nil, errors.New("multiple client.Object arguments provided")
				} else if opts.Objects != nil {
					return nil, errors.New(errObjectAndObjects)
				} else if util.IsNil(obj) {
					return nil, errors.New(
						"provided client.Object is nil or has a nil underlying value")
				} else {
					opts.Object = obj
				}
				continue
			}
		}

		if includeObjects {
			// Check for Objects
			if objs, ok := util.AsSliceOfObjects(arg); ok {
				if opts.Objects != nil {
					return nil, errors.New("multiple []client.Object arguments provided")
				} else if opts.Object != nil {
					return nil, errors.New(errObjectAndObjects)
				} else if util.ContainsNil(objs) {
					return nil, errors.New(
						"provided []client.Object contains an element that is nil or has a nil underlying value")
				} else {
					opts.Objects = objs
				}
				continue
			}
		}

		if includeTemplate {
			// Check for Template
			if str, ok := arg.(string); ok {
				if opts.Template != "" {
					return nil, errors.New("multiple template arguments provided")
				} else {
					var err error
					opts.Template, err = ProcessTemplate(str)
					if err != nil {
						return nil, err
					}
				}
				continue
			}
		}

		// Check for Bindings
		if bindings, ok := util.AsMapStringAny(arg); ok {
			opts.Bindings = util.MergeMaps(opts.Bindings, bindings)
			continue
		}

		return nil, fmt.Errorf("unexpected argument type: %T", arg)
	}

	return opts, nil
}

// applyDefaults applies defaults to the given options where needed.
func applyDefaults(defaults, opts *Options) *Options {
	// Nil checks
	if defaults == nil {
		return opts
	} else if opts == nil {
		return defaults
	}

	// Default durations
	if opts.Timeout == 0 {
		opts.Timeout = defaults.Timeout
	}
	if opts.Interval == 0 {
		opts.Interval = defaults.Interval
	}

	// Merge bindings
	opts.Bindings = util.MergeMaps(defaults.Bindings, opts.Bindings)

	return opts
}

// ParseAndApplyDefaults parses variable arguments into an Options struct
// and applies defaults where needed.
func ParseAndApplyDefaults(
	defaults *Options,
	includeDurations bool,
	includeObject bool,
	includeObjects bool,
	includeTemplate bool,
	args ...interface{},
) (*Options, error) {
	opts, err := parse(includeDurations, includeObject, includeObjects, includeTemplate, args...)
	if err != nil {
		return nil, err
	}
	return applyDefaults(defaults, opts), nil
}

// RequireDurations requires options Timeout and Interval to be provided.
func RequireDurations(opts *Options) error {
	if opts == nil {
		return errors.New(errNil)
	}
	if opts.Timeout == 0 {
		return errors.New(errRequired + ": Timeout (string or time.Duration)")
	}
	if opts.Interval == 0 {
		return errors.New(errRequired + ": Interval (string or time.Duration)")
	}
	return nil
}

// RequireTemplate requires option Template to be provided.
func RequireTemplate(opts *Options) error {
	if opts == nil {
		return errors.New(errNil)
	}
	if len(opts.Template) == 0 {
		return errors.New(errRequired + ": Template (string)")
	}
	return nil
}

// RequireTemplateObject requires options Template or Object to be provided.
func RequireTemplateObject(opts *Options) error {
	if opts == nil {
		return errors.New(errNil)
	}
	if len(opts.Template) == 0 && opts.Object == nil {
		return errors.New(errRequired + ": Template (string) or Object (client.Object)")
	}
	return nil
}

// RequireTemplateObjects requires options Template or Objects to be provided.
func RequireTemplateObjects(opts *Options) error {
	if opts == nil {
		return errors.New(errNil)
	}
	if len(opts.Template) == 0 && opts.Objects == nil {
		return errors.New(errRequired + ": Template (string) or Objects ([]client.Object)")
	}
	return nil
}

// RequireTemplateObjectObjects requires options Template, Object, or Objects to be provided.
func RequireTemplateObjectObjects(opts *Options) error {
	if opts == nil {
		return errors.New(errNil)
	}
	if len(opts.Template) == 0 && opts.Object == nil && opts.Objects == nil {
		return errors.New(errRequired + ": Template (string), Object (client.Object), or Objects ([]client.Object)")
	}
	return nil
}
