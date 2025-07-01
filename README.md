# Check hosts and version with `/git` endpoint
## `hostsc` stands for 'Hosts Checker'

Check multiple hosts version "blazingly fast"


The hosts must have a `/git` endpoint, like:

```sh
curl myhost.com/git
```


If the hosts is a React web application, it needs to have a console.log with this pattern:

```javascript
console.log("HEAD VERSION: .*");
```

Example:

```javascript
console.log("HEAD VERSION: ", process.env.REACT_APP_GIT_HEAD_HASH);
```
