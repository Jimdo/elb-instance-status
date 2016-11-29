# 0.5.0 / 2016-11-29

  * Add line prefixing to see which check logs lines
  * Log failing checks

# 0.4.1 / 2016-10-11

  * Push builds to GH releases

# 0.4.0 / 2016-08-04

  * `stderr` output will be sent to `stderr` of the program all the time which helps debugging failing commands
  * `stdout` can be attached to `stdout` of the program when `--verbose` flag is passed
  * All commands are now executed using `bash -e -o pipefail -c '<command>'` to make them more reliable and let them fail fast in case of an error

# 0.3.0 / 2016-07-22

  * make check interval and config refresh interval configurable
  * enforce checks to quit before they are started again
  * support URLs as check source

# 0.2.1 / 2016-06-06

  * Vendored dependencies

# 0.2.0 / 2016-06-06

  * Expose metrics about checks for prometheus
  * **Breaking change:** The configuration format changed during this release!

# 0.1.1 / 2016-06-03

  * Bake in the version when using gobuilder

# 0.1.0 / 2016-06-03

  * Added documentation
  * Initital version
