Logger Proxy API
================

## Consts (in decreasing order of severity):
 * FATAL
 * ERROR
 * WARN
 * INFO
 * DEBUG
 * TRACE

## Functions:
 * log.SetLogLevel(<const from above>):
    Log messages below this level of severity will be ignored.
    Default Log Level is INFO.

 * log.Fatal(string, vals... interface{}):
    prevents the program from continuing
    i.e. can't allocate additional memory

 * log.Error(string, vals... interface{}):
    non-fatal, but prevents valid execution
    i.e. can't connect to a database, complete a function call, open file, invalid format

 * log.Warn(string, vals... interface{}):
    looks unusual, but does not clearly prevent execution

 * log.Info(string, vals... interface{}):
    Least severe message that a sysadmin would be interested in
    i.e. server request logs

 * log.Debug(string, vals... interface{}):
    high level info for a developer. more than what a sysadmin would typically want

 * log.Trace(string, vals... interface{}):
    excruciating detail.

 * log.SetOutput(io.Writer):
    Indicates the location for log messages to be written.
    Default is stdout.

## Flags:

    These package-level flags are provided to disable expensive code when the code is only needed at
	a lower severity than the logger is set at:
        IsError
        IsWarn
        IsInfo
        IsDebug
        IsTrace

	example usage:
         if log.IsDebug {
             ...
         }

## Output will look like:
	"timestamp•LOG_LEVEL•filename.go•linenumber•output"
