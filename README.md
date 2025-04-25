# Notificator (MVP)

**⚠️ This project is an MVP – Minimum Viable Product.**  
It’s a proof-of-concept built to validate the idea of centralizing multi-platform notifications and triggering outbound voice calls via Webitel. Feedback and contributions are welcome!

## Overview

`notificator` is a lightweight Go CLI application designed for support teams, especially during night shifts. It consolidates alerts and notifications from multiple messaging platforms and webhooks, then routes them to [Webitel](https://www.webitel.com/) dialer for immediate voice call alerts.

## Features

- ✅ Listens for messages as a **logged-in user** from:
    - **Skype**
    - **Microsoft Teams**
    - **Telegram**
- ✅ Accepts **incoming webhooks** (e.g. from monitoring tools like Prometheus, Grafana, etc., or from JIRA)
- ✅ Forwards notifications to the **Webitel dialer** to initiate outbound calls
- ✅ Ideal for **on-call systems** or **night-shift support** workflows

## Use Case

This tool is perfect for environments where critical alerts come from multiple platforms and need to be escalated immediately — especially during off-hours. By funneling all messages into a single system and triggering phone calls, you can ensure nothing is missed.
