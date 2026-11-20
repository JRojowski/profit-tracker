# The Assistant reads data through backend functions and does not calculate

The Assistant uses a language model to answer the questions of the user about the investments. The backend gives the model a fixed list of read-only functions. The model selects a function and its arguments, and then it describes the result. The model does not write SQL and does not calculate a number.

## Why

- The tool calculates its results in Go code, not in the database. For example, Realized Profit comes from the Ledger (ADR 0001). A model that writes SQL calculates such a result again with different rules, and the answer does not agree with the screens.
- Language models make arithmetic errors. The first model is a free model, and a free model makes more of them.
- A wrong number about money looks the same as a correct number. The user cannot see the error in the answer.

## The rule

1. Each function is read-only and returns a result that the backend calculates.
2. The model gets data only from these functions.
3. Each number in an answer comes from a function result. The model does not add, subtract or convert numbers.
4. If no function fits the question, the Assistant says that it cannot answer. It does not estimate.
5. With each answer, the Assistant shows the functions and the arguments that the model used.

## Considered options

- **The model writes SQL.** It answers more questions with no new code. We rejected it because of the first reason above.
- **The backend sends all data in the prompt.** It needs no functions. We rejected it because the model must then calculate, and because each question sends all data to the provider.

## Consequences

- The Assistant answers only the questions that a function covers. A new type of question needs a new function.
- The provider of the model gets only the question and the results of the selected functions.
