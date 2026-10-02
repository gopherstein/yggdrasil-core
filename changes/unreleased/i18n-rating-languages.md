### Added

- Ratings by language: rating a model asks how well it worked for you in a language, your App language unless you choose another or not to say. A model card shows its community ratings by language, your App language first, and with community ratings on, Auto counts them when choosing a model for an answer in that language. The API's `PUT /models/{id}/rating` takes `language`, and `/ratings/community` returns each model's `languages`.
