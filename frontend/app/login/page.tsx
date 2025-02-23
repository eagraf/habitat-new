'use client'

import React, { Suspense } from 'react';
import { useRouter } from 'next/navigation';
import styles from './login.module.css';

const LoginInternal = () => {
    useRouter();

    return (
        <div className={styles.loginBody}>
            <div className={styles.loginContainer}>
                <form className={styles.loginForm} action="/habitat/oauth/login" method="post"> 
                <h2>Login</h2>
                <div className={styles.formGroup}>
                    <label htmlFor="handle">Handle:</label>
                    <input
                        type="text"
                        id="handle"
                        name="handle"
                    />
                </div>
                <button className={styles.loginButton} type="submit">Login</button>
                </form>
            </div>
        </div>
    );
};

const Login = () => {
    // TODO redirect to home if already logged in
    return (
        <Suspense fallback={<div>Loading...</div>}>
            <LoginInternal />
        </Suspense>
    );
};

export default Login;