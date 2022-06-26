# go-qap
To solve the document naming convention problem.
---

Implementation of CERN's quality assurance plan (QAP) in pure Go.

_This is a WIP._


### Motivation
Startups in the business of mechanical engineering have limited choices
when it comes to choosing cost effective PLM software.

In particular there is a real and immediate need to document deliverables
and keep them organized in an electronic file repository.

This repository seeks to fully solve the problem regarding 
**document naming conventions**. This may seem like a trivial task but it is something
that has been given a great deal of thought, not least by [those in charge of
the LHC project at CERN](https://edms.cern.ch/ui/file/103547/1.1/QAp202rev1-2.pdf).

### BoltQAP

![BoltQAP](https://user-images.githubusercontent.com/26156425/175828863-e1324e25-7b4a-4f12-964d-db4c6e11fa8c.png)

BoltQAP is a basic implementation of a document naming application which runs in a server. It is in the [`cmd/boltqap`](./cmd/boltqap/) directory.

Get it by running

```sh
go install github.com/soypat/go-qap/cmd/boltqap@latest
```

