'use client'

import React, { useState, FormEvent } from 'react';
import axios from 'axios';
import Cookies from 'js-cookie';
import './login.css';

const Login = () => {
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');

    const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        // Handle login logic here
        console.log('Email:', email);
        console.log('Password:', password);

        try {
            const response = await axios.post('http://habitat:3001/xrpc/com.atproto.server.createSession', {
                email: email,
                handle: "ethan.test",
                password: password,
                identifier: "did:plc:3wff4wcyfnjsxd2d2yz3azrj"
              }, {
                headers: {
                  'Content-Type': 'application/json',
                },
            });
            console.log(response.data);

            const { accessJwt, refreshJwt } = response.data;


            // Set the access token in a cookie
            Cookies.set('access_token', accessJwt, { expires: 7 });
            Cookies.set('refresh_token', refreshJwt, { expires: 7 });

          } catch (err) {
            console.error(err);
          }
    };

    return (
        <div className="login-container">
            <form className="login-form" onSubmit={handleSubmit}>
                <h2>Login</h2>
                <div className="form-group">
                    <label htmlFor="email">Email:</label>
                    <input
                        type="email"
                        id="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        required
                    />
                </div>
                <div className="form-group">
                    <label htmlFor="password">Password:</label>
                    <input
                        type="password"
                        id="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        required
                    />
                </div>
                <button type="submit">Login</button>
            </form>
        </div>
    );
};

export default Login;