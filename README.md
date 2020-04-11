## 
Download a file as parts use basic `Range` header of HTTP protocol

## Run
1. Change `targetFileUrl` and `output` variables as you want.
1. Run `go run main.go` to download. The result should be:

```
Download file as parts
Path  /photos/853199/pexels-photo-853199.jpeg
Host  images.pexels.com
Size:  23850819
Down load ranges:  [0 4770162 4770163 9540325 9540326 14310488 14310489 19080651 19080652 23850819]
Part: 1 		 Time: 2.645663 seconds 
Part: 4 		 Time: 2.727898 seconds 
Part: 3 		 Time: 2.824624 seconds 
Part: 2 		 Time: 3.145211 seconds 
Part: 0 		 Time: 3.332437 seconds 
Merge files.		 Time: 0.247856 seconds
Output  ./wall_pager.jpg


```