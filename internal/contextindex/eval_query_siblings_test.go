package contextindex

const evalActiveHelpSource = `package cobra

// AppendActiveHelp adds the string; see GetActiveHelpConfig.
func AppendActiveHelp(compArray []string, activeHelpStr string) []string {
	return append(compArray, activeHelpStr)
}

// GetActiveHelpConfig returns the value of the ActiveHelp environment variable.
func GetActiveHelpConfig(cmd string) string {
	return os.Getenv(activeHelpEnvVar(cmd))
}

func activeHelpEnvVar(name string) string {
	return name + "_ACTIVE_HELP"
}

func TestGetActiveHelpConfig() {}
`

const evalViewsSource = `class View:
    """Call :meth:` + "`as_view`" + ` to create a view function."""

    def dispatch_request(self):
        raise NotImplementedError()

    def as_view(cls, name):
        def view(**kwargs):
            return self.dispatch_request(**kwargs)
        return view


class MethodView(View):
    """app.add_url_rule("/counter", view_func=CounterAPI.as_view("counter"))"""

    def dispatch_request(self, **kwargs):
        meth = getattr(self, request.method.lower(), None)
        return meth(**kwargs)


def async_to_sync(func):
    return func


def ensure_sync(func):
    return async_to_sync(func)
`
