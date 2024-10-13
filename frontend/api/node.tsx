import axios from 'axios';
import Cookies from 'js-cookie';
import * as node from '../types/api';

export const getNode = async (): Promise<node.GetNodeResponse> => {
    try {
        const accessToken = Cookies.get('access_token');
        if (!accessToken) {
            throw new Error('No access token found');
        }

        const response = await axios.get(`${window.location.origin}/habitat/api/node`, {
            headers: {
                'Authorization': `Bearer ${accessToken}`,
                'Content-Type': 'application/json',
            },
        });

        return response.data;
    } catch (error) {
        console.error('Error fetching node data:', error);
        throw error;
    }
};

export const installApp = async (appInstallation: any) => {
    try {
        const accessToken = Cookies.get('access_token');
        if (!accessToken) {
            throw new Error('No access token found');
        }

        const response = await axios.post(
            `${window.location.origin}/habitat/api/node/users/0/apps`,
            appInstallation,
            {
                headers: {
                    'Authorization': `Bearer ${accessToken}`,
                    'Content-Type': 'application/json',
                },
            }
        );

        return response.data;
    } catch (error) {
        console.error('Error installing app:', error);
        throw error;
    }
};
