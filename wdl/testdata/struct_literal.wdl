version 1.2

struct Point {
  Int x
  Int y
}

task struct_task {
  Point p = Point { x: 1, y: 2 }
  Int px = p.x

  command {}
}
