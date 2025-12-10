I am unable to generate a PDF file directly. However, I can provide the comprehensive content in a structured, document-like format using Markdown, consolidating the project scope, technical approach, constraints, and objectives discussed.

***

# Technical Report: Carnival Casino Operations Snowflake ML POC

**Project Title:** Voyage-Aware Slot Denomination & Mix Optimizer  
**Consulting Team:** BlueCloud & Snowflake

## 1. Executive Summary and Core Objectives

The primary goal of this Proof of Concept (POC) is to transition the Carnival Casino Operations team from a manual slot floor tuning process to a repeatable, data-driven method to maximize casino profitability. The solution will leverage Snowflake's cutting-edge native features to develop a recommendation system for optimizing slot machine denominations and minimum bets based on guest demographics and usage patterns.

| Metric | Detail | Citation |
| :--- | :--- | :--- |
| Primary Goal | Increase overall casino profitability by optimizing slot denomination mix (penny vs. mid/high denomination). | |
| Key Metric of Success | Uplift in Daily Net Win Index (normalized measure comparing machine net win to ship average). | |
| Actionable Deliverable | A Pre-Voyage Change List that Operations can apply between sailings. | |

### Phased Implementation Approach

The project is structured in three phases to build capability and confidence:

1. Phase 1: Correlation Analysis to identify key performance drivers influencing mid/high denomination slot performance (e.g., player tiers, voyage length).
2. Phase 2: Establish Baseline Slot Mix per ship for average voyages, reducing operational complexity.
3. Phase 3: Generate Voyage-Specific Recommendations (e.g., "Convert X penny slots to mid/high denomination") with projected uplift.

## 2. POC Scope Constraints and Operational Practicability

The POC has defined constraints to ensure the solution is feasible and executable by the client team (Milen's team).

| Constraint Type | Detail | Citation |
| :--- | :--- | :--- |
| Operational Feasibility | Denomination adjustments are limited to 10-15 machines per ship. | |
| Timing | Changes must occur Pre-Voyage; no mid-cruise changes are allowed to prevent guest issues. | |
| Data Recency | Use recent data only (about the last year) (~100 GB total) as older behavior is less predictive, especially post-COVID. | |
| Data Access | All work must occur inside Carnival's Snowflake (CCL/Holland) in a separate, brand-isolated Global Gaming area. No cross-brand blending or external enrichments are in scope for the POC. | |
| Model Requirement | The model must have Trust/Explainability, ensuring the factors behind each recommendation make operational sense. | |

## 3. Technical Approach and MLOps Pipeline

The technical strategy centers on leveraging Carnival's existing Snowflake environment for a robust MLOps framework.

### A. Core Architecture and Tools

The solution relies entirely on Snowflake's native features:

1. Preparation and Transformation: Data is prepared and transformed using Snowflake ML APIs.
2. Feature Management: Features are managed within the Snowflake Feature Store.
3. Model Development: Models are developed using Snowflake ML functions or custom development leveraging Snowpark.
4. Model Deployment: Trained models are registered in the Snowflake Model Registry for inference and management.
5. Monitoring: Observability Monitoring for ML Models tracks performance using monitoring logs.

### B. MLOps Governance

The project follows a framework of Unified Governance across five stages:

* Develop & Iterate
* Orchestrate & Automate
* Manage
* Deploy & Serve
* Monitor

## 4. Execution Steps and Technical Workflow

The 8-week POC is broken down into structured technical steps.

### Step 1: Data Collection and Preparation (Weeks 1-2)

* Ingestion: Ingest member demographics and historic slot data files into Snowflake, leveraging storage integration.
* Cleaning: Handle missing values and perform deep dive analysis to identify data patterns based on business context.
* Encoding: Convert categorical data (like membership tier) into numerical formats for ML models.
* Splitting: Divide the prepared dataset into Training, Validation, and Test sets.
* Data Sources: Utilize slot telemetry (occupancy, spins, coin-in/out), player ratings/cohorts (value tiers, simple demographics), voyage-level aggregates (net win, average bet), and past denomination change outcomes.

### Step 2: Feature Engineering and Model Training (Weeks 3-5)

* Feature Engineering: Extract new, basic features; convert transaction series into summary statistics/aggregations; and apply scale adjustments to numerical features to prevent dominance.
* Training: Train the chosen model(s) (classification or regression) using the Training data to establish the relationship between demographics and the target (denomination/bet).

### Step 3: Model Evaluation and Documentation (Weeks 6-8)

* Evaluation: Use the trained model to make predictions on the unseen Test data. Compare the model's performance (e.g., accuracy, RMSE) against a simple baseline (e.g., recommending the most popular denomination).
* Success Verification: Confirm the model meets the required Explainability and Operational Practicability constraints.
* Final Output: Produce POC documentation detailing the data, model, performance metrics, and recommendations for future steps (like exploring more complex models or integrating real-time features).

## 5. Timeline and Commercials

| Component | Detail | Citation |
| :--- | :--- | :--- |
| POC Duration | 8 weeks. | |
| Team | Data Science Team (including AI/ML Engineer Selma Seljubac and LLM Engineer Cagan Kiper). | |
| Cost | The Total Cost to Carnival for the Point of View is $0, as BlueCloud and Snowflake are investing in the POC. | |
