# Brain-Freeze

Brain-Freeze is a Cortex CLI to debug the On-Prem installation offered by Cortex.

## Installation

We publish the latest version of this binary on every code change to https://github.com/cortexapps/brain-freeze/releases/tag/latest
The binaries are available for both `MacOS` and `Linux` operating systems.

To fetch, just download the binary and add them to your PATH.

```bash
# Make sure to use the binary compatible with your OS!

wget https://github.com/cortexapps/brain-freeze/releases/download/latest/brain-freeze-latest-darwin-amd64.tar.gz
tar -xvf brain-freeze-latest-darwin-amd64.tar.gz
```


## Usage

The CLI comes with helpers built-in to aid through the usage. Can run any command with `--help` to get more information.

```bash
brain-freeze --help
```

Generally you will want to run:

- `brain-freeze k8s dump`: Dumps installation information
- `brain-freeze k8s logs`: Dumps pod logs

Both of these will dump to the `data/` directory in the current
working directory, which you can tar or zip and send to Cortex support.
