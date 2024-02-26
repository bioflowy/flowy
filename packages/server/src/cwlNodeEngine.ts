export const fileContent: string = "\"use strict\";\nprocess.stdin.setEncoding(\"utf8\");\nvar incoming = \"\";\nprocess.stdin.on(\"data\", function(chunk) {\n  incoming += chunk;\n  var i = incoming.indexOf(\"\\n\");\n  if (i > -1) {\n    try{\n      var fn = JSON.parse(incoming.substr(0, i));\n      incoming = incoming.substr(i+1);\n      const str = JSON.stringify(require(\"vm\").runInNewContext(fn, {})) + \"\\n\"\n      process.stderr.write(`strlen=${str.length}\\n`);\n      process.stdout.write(str,(err) => {\n        process.exit(0);\n      });\n      process.stdout.end();\n    }\n    catch(e){\n      console.error(e)\n      process.exit(1);\n    }\n  }\n});\n";
