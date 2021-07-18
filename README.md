# constructor
A tiny tool to make data-parsing and constructing deadly easy.
please check the unit tests for usage.

## All supported converters:

- from(field)
- split(sep) / split(sep, converter)
- map(converter)
- dict(field)
- obj(type)
- group(field) / group(field, reduce)
- sort(field) / sort(field, desc)

## Performance tips

due to the heavy usage of reflection, it performs much bad than a hand-written loader,
so it's not a good idea to use it to handle super large tables.