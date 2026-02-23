import App from "@/App.vue";
import "@/assets/style.css";
import "@/styles/modal.css";
import router from "@/router";
import i18n from "@/locales";
import naive from "naive-ui";
import { createApp } from "vue";
import { createPinia } from "pinia";

const app = createApp(App);
const pinia = createPinia();

app.use(router).use(naive).use(i18n).use(pinia).mount("#app");
