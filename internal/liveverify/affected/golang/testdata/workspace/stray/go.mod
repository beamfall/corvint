module example.com/workspace/stray

go 1.26

// V1-0867: an unlisted module that requires an observed one stays a frontier.
require example.com/workspace/core v0.0.0
