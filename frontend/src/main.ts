import { mount } from 'svelte';
import App from './App.svelte';
import './app.css';
import { initSettings } from './stores/settings';

void initSettings().then(() => mount(App, { target: document.getElementById('app')! }));
