version 1.2

workflow if_scatter_nested_wf {
  input {
    Boolean do_scatter
    Array[Int] scatter_range
  }

  # if { scatter { Int } } should produce Array[Int]? (not Array[Int?])
  if (do_scatter) {
    scatter (i in scatter_range) {
      Int result = i * 2
    }
  }

  # scatter { if { Int } } should produce Array[Int?] (not Array[Int]?)
  scatter (j in scatter_range) {
    if (do_scatter) {
      Int inner = j + 1
    }
  }
}
