version 1.2
import "lib.wdl"

workflow main {
  call greet { input: person = "Alice" }
}
