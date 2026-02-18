import App from "@/App.vue";
import "@/assets/style.css";
import "@/styles/modal.css";
import router from "@/router";
import i18n from "@/locales";
import naive from "naive-ui";
import { createApp } from "vue";

createApp(App).use(router).use(naive).use(i18n).mount("#app");
