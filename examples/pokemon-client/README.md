# Pokemon Client

This is an example of how babyapi can be used just to create a client for an external API, like the [PokeAPI](https://pokeapi.co/docs/v2).

Although it works with the default commands, this example adds an extra command to better demonstrate the Pokemon struct since the default commands will print the raw JSON response.

## How To

Use the custom `get` command to pretty-print Pokemon details
```shell
> go run main.go get pikachu
Name: pikachu
Types: [electric]
2 Abilities
105 Moves
```

Use the default `client get` command to print raw JSON data
```shell
go run main.go client --address "https://pokeapi.co" pokemon get pikachu
```
