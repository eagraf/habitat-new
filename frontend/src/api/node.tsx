import axios from 'axios';
import Cookies from 'js-cookie';
import * as node from '../../types/api';

export const getNode = async (): Promise<node.GetNodeResponse> => {
  try {
    const response = await axios.get(`${window.location.origin}/habitat/api/node`, {
      headers: {
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

export const getAvailableApps = async (): Promise<any[]> => {
  try {
    const response = await axios.get(`${window.location.origin}/habitat/api/app_store/available_apps`);
    return response.data;
  } catch (error) {
    console.error('Error fetching available apps:', error);
    throw error;
  }
};
