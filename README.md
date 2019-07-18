# Coherent Line Drawing - OpenFaaS function

[![license](https://img.shields.io/github/license/mashape/apistatus.svg?style=flat)](./LICENSE)

For more info about the implementation details check the project source code at https://github.com/esimov/colidr.

### Local usage
To run the function locally you have to make sure OpenFaaS is up and running. Read the official documentation for more help. https://docs.openfaas.com/

Clone the repository:
```bash
$ git clone https://github.com/esimov/openfaas-coherent-line-drawing
```

#### Build
```bash 
$ faas-cli template pull https://github.com/alexellis/opencv-openfaas-template
$ faas-cli build -f stack.yml --gateway=http://<GATEWAY-IP>
```

#### Deploy
```bash 
$ faas-cli deploy -f stack.yml --gateway=http://<GATEWAY-IP>
```
You can access the UI on the url provided to `--gateway`. 

**Note:** in case of large images you need to increase `write_timeout` in stack.yml.

### Results
After deployment the `coherent-line-drawing` function will show up in the function list. You have to provide an image URL then hit invoke. This will generate a contoured, sketch-liked image.

![image](https://user-images.githubusercontent.com/883386/61373248-fd09f500-a8a1-11e9-9bb2-55aa3f0722e6.png)

You can also provide different values as query parameters. The follwing parameters are supported:

| Flag | Default value | Description |
| --- | --- | --- |
| `aa` | false | Anti aliasing |
| `bl` | 3 | New height |
| `di` | 1 | Number of FDoG iteration |
| `ei` | 2 | Number of Etf iteration |
| `k` | 2 | Etf kernel |
| `rho` | 0.98 | Rho |
| `sc` | 1 | Sigma C |
| `sm` | 3 | Sigma M |
| `sr` | 2.6 | Sigma R |
| `tau` | 0.98 | Tau |

Below is an example with query parameters you can try out:
```bash
https://user-images.githubusercontent.com/883386/61370913-30e21c00-a89c-11e9-8edf-f4b59b59793c.jpg?k=2&sr=2.9&sm=3.5&tau=0.999&aa=1&ei=2&di=1
```

| Input | Output
|:--:|:--:|
| ![tiger_source](https://user-images.githubusercontent.com/883386/61370913-30e21c00-a89c-11e9-8edf-f4b59b59793c.jpg) | ![tiger_dest](https://user-images.githubusercontent.com/883386/60795443-5cfeee00-a174-11e9-9fd4-6ceb9a02ca21.png) |
| ![patio_source](https://user-images.githubusercontent.com/883386/61370926-37709380-a89c-11e9-8b2c-157482c27192.jpg) | ![patio_dest](https://user-images.githubusercontent.com/883386/60726045-40c83a80-9f43-11e9-9d53-7f190889e4bc.jpg) |
| ![people_source](https://user-images.githubusercontent.com/883386/61370965-4bb49080-a89c-11e9-9ec6-e5fde965a046.jpg) | ![people_dest](https://user-images.githubusercontent.com/883386/60795438-5c665780-a174-11e9-8c8a-365bd8eda329.png) |
| ![starry_night_source](https://user-images.githubusercontent.com/883386/61370917-32abdf80-a89c-11e9-98ae-7c06635066bf.jpg) | ![starry_night_dest](https://user-images.githubusercontent.com/883386/60795440-5c665780-a174-11e9-9804-d5e56d0c49e7.png) |


## License

Copyright © 2019 Endre Simo

This project is under the MIT License. See the LICENSE file for the full license text.

