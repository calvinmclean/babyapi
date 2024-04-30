# Nested Example

This implements an API where the root resource is an `Artist` which can have `Albums` and `MusicVideos`. Then, `Albums` can have `Songs`.

It demonstrates:
  - APIs with nested/related resources
  - Custom `ResponseWrapper` which allows a Song response to show Album details
  - `extensions.HATEOAS` to easily add hypermedia linking to resources so a user can discover Albums by looking at an Artist and then discover Songs for the Album.


## Try it out!

Run the server
```shell
go run main.go serve
```

Create an Artist
```shell
⟩ go run main.go client \
  artists post \
  --data '{"name":"artist1"}'
```
```json
{
    "id": "coinqaon1e4crs6ambu0",
    "links": {
        "Albums": "/artists/coinqaon1e4crs6ambu0/albums",
        "self": "/artists/coinqaon1e4crs6ambu0"
    },
    "name": "artist1"
}
```

Create an Album
```shell
⟩ go run main.go client \
  albums post \
  --artists-id coinqaon1e4crs6ambu0 \
  --data '{"title":"album1"}'
```
```json
{
    "id": "coinqq8n1e4crs6ambug",
    "links": {
        "Songs": "/artists/coinqaon1e4crs6ambu0/albums/coinqq8n1e4crs6ambug/songs",
        "self": "/artists/coinqaon1e4crs6ambu0/albums/coinqq8n1e4crs6ambug"
    },
    "title": "album1"
}
```
