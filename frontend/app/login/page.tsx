'use client'

import React, { useState, FormEvent } from 'react';
import axios from 'axios';
import Cookies from 'js-cookie';
import './login.css';

const Login = () => {
    const [handle, setHandle] = useState('');
    const [password, setPassword] = useState('');

    const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        // Handle login logic here
        console.log('Handle:', handle);
        console.log('Password:', password);

        try {
            const response = await axios.post(`${window.location.origin}/habitat/api/node/login`, {
                password: password,
                identifier: handle,
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
                    <label htmlFor="handle">Handle:</label>
                    <input
                        type="text"
                        id="handle"
                        value={handle}
                        onChange={(e) => setHandle(e.target.value)}
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