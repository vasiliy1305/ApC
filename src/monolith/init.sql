CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL
);

CREATE TABLE movies (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    rating FLOAT NOT NULL
);

CREATE TABLE movie_genres (
    movie_id INT REFERENCES movies(id),
    genre TEXT NOT NULL
);

CREATE TABLE payments (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    amount FLOAT NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    plan_type TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL
);
