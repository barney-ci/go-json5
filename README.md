# barney.ci/go-json5

This library implements a parser that supports a subset of the JSON5 specification.

## Features

It actually works. I'm not even kidding, I tried alternatives and they all
failed on real-world parsing scenarios when trying to deal with comments.

Everything in the JSON5 specification is supported _except_ for Infinity and
NaN. This is because this library translates a JSON5 document to JSON, where
both values are non-representable. A proper parser needs to be implemented
in order to support that use-case.

## Usage

```
go get barney.ci/go-json5
```
