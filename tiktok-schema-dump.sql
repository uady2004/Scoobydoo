--
-- PostgreSQL database cluster dump
--

\restrict O4D4gEVKC6bKlQEiwu7ZArtuDHXKuiDQCPGD6fZcdRt6IduPnlarwHn2FlT2lk0

SET default_transaction_read_only = off;

SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;

--
-- Roles
--

CREATE ROLE postgres;
ALTER ROLE postgres WITH SUPERUSER INHERIT CREATEROLE CREATEDB LOGIN REPLICATION BYPASSRLS PASSWORD 'SCRAM-SHA-256$4096:a0T5fnF3lb4oR8P2pp+NlA==$DAejE78nbvJr6n3kdjafbSLvORn3c4vxM1AawKDwrtM=:BOdaD/7YzZ33b98Ye19GNCO9zzUv9hbo993CisqhnwE=';

--
-- User Configurations
--








\unrestrict O4D4gEVKC6bKlQEiwu7ZArtuDHXKuiDQCPGD6fZcdRt6IduPnlarwHn2FlT2lk0

--
-- Databases
--

--
-- Database "template1" dump
--

\connect template1

--
-- PostgreSQL database dump
--

\restrict F0vu6de1AUngwTtJP5ghfyvXJXaUobrBGtkwtVTipdaJ0z141eIb90Ry0yV9mif

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- PostgreSQL database dump complete
--

\unrestrict F0vu6de1AUngwTtJP5ghfyvXJXaUobrBGtkwtVTipdaJ0z141eIb90Ry0yV9mif

--
-- Database "postgres" dump
--

\connect postgres

--
-- PostgreSQL database dump
--

\restrict Y99OnIqj9dn53v13TrJUpAFhXfo9V5hsnaALHABIioBv3isXJOdWe4pxlk3lLPO

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- PostgreSQL database dump complete
--

\unrestrict Y99OnIqj9dn53v13TrJUpAFhXfo9V5hsnaALHABIioBv3isXJOdWe4pxlk3lLPO

--
-- Database "tiktok" dump
--

--
-- PostgreSQL database dump
--

\restrict cnToOV7opdhHMnyn45CufGh1vnBM11h9BBAHka3e7fBuppBGB6kAPWuBLojLB8Q

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok OWNER TO postgres;

\unrestrict cnToOV7opdhHMnyn45CufGh1vnBM11h9BBAHka3e7fBuppBGB6kAPWuBLojLB8Q
\connect tiktok
\restrict cnToOV7opdhHMnyn45CufGh1vnBM11h9BBAHka3e7fBuppBGB6kAPWuBLojLB8Q

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- PostgreSQL database dump complete
--

\unrestrict cnToOV7opdhHMnyn45CufGh1vnBM11h9BBAHka3e7fBuppBGB6kAPWuBLojLB8Q

--
-- Database "tiktok_admin" dump
--

--
-- PostgreSQL database dump
--

\restrict FZHP4aiW9NsBr5gUvBaSBZ7bHYJIfcepBVz59E6NbWVjUJHT9Qe5GMrZWKZHlYH

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_admin; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_admin WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_admin OWNER TO postgres;

\unrestrict FZHP4aiW9NsBr5gUvBaSBZ7bHYJIfcepBVz59E6NbWVjUJHT9Qe5GMrZWKZHlYH
\connect tiktok_admin
\restrict FZHP4aiW9NsBr5gUvBaSBZ7bHYJIfcepBVz59E6NbWVjUJHT9Qe5GMrZWKZHlYH

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: admin_role; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.admin_role AS ENUM (
    'super_admin',
    'admin',
    'moderator',
    'analyst',
    'support'
);


ALTER TYPE public.admin_role OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: admin_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.admin_users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    username text NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    role public.admin_role DEFAULT 'support'::public.admin_role NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    last_login_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.admin_users OWNER TO postgres;

--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.audit_logs (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    admin_id uuid NOT NULL,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id text,
    old_values jsonb,
    new_values jsonb,
    ip_address text,
    user_agent text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.audit_logs OWNER TO postgres;

--
-- Name: moderation_actions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.moderation_actions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    admin_id uuid NOT NULL,
    target_id uuid NOT NULL,
    target_type text NOT NULL,
    action text NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    expires_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.moderation_actions OWNER TO postgres;

--
-- Data for Name: admin_users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.admin_users (id, username, email, password_hash, role, is_active, last_login_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: audit_logs; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.audit_logs (id, admin_id, action, entity_type, entity_id, old_values, new_values, ip_address, user_agent, created_at) FROM stdin;
\.


--
-- Data for Name: moderation_actions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.moderation_actions (id, admin_id, target_id, target_type, action, reason, expires_at, created_at) FROM stdin;
\.


--
-- Name: admin_users admin_users_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.admin_users
    ADD CONSTRAINT admin_users_email_key UNIQUE (email);


--
-- Name: admin_users admin_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.admin_users
    ADD CONSTRAINT admin_users_pkey PRIMARY KEY (id);


--
-- Name: admin_users admin_users_username_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.admin_users
    ADD CONSTRAINT admin_users_username_key UNIQUE (username);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: moderation_actions moderation_actions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.moderation_actions
    ADD CONSTRAINT moderation_actions_pkey PRIMARY KEY (id);


--
-- Name: idx_audit_logs_admin_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_audit_logs_admin_id ON public.audit_logs USING btree (admin_id, created_at DESC);


--
-- Name: idx_audit_logs_created; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_audit_logs_created ON public.audit_logs USING btree (created_at DESC);


--
-- Name: idx_audit_logs_entity; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_audit_logs_entity ON public.audit_logs USING btree (entity_type, entity_id);


--
-- Name: idx_moderation_target; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_moderation_target ON public.moderation_actions USING btree (target_id, target_type);


--
-- Name: audit_logs audit_logs_admin_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_admin_id_fkey FOREIGN KEY (admin_id) REFERENCES public.admin_users(id);


--
-- Name: moderation_actions moderation_actions_admin_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.moderation_actions
    ADD CONSTRAINT moderation_actions_admin_id_fkey FOREIGN KEY (admin_id) REFERENCES public.admin_users(id);


--
-- PostgreSQL database dump complete
--

\unrestrict FZHP4aiW9NsBr5gUvBaSBZ7bHYJIfcepBVz59E6NbWVjUJHT9Qe5GMrZWKZHlYH

--
-- Database "tiktok_analytics" dump
--

--
-- PostgreSQL database dump
--

\restrict JOxwLi7JScCzCxVUOOmZ8S33AP3vXHZhxS7dlABqhi03ry4zjdcX3tfwvrqxWnb

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_analytics; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_analytics WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_analytics OWNER TO postgres;

\unrestrict JOxwLi7JScCzCxVUOOmZ8S33AP3vXHZhxS7dlABqhi03ry4zjdcX3tfwvrqxWnb
\connect tiktok_analytics
\restrict JOxwLi7JScCzCxVUOOmZ8S33AP3vXHZhxS7dlABqhi03ry4zjdcX3tfwvrqxWnb

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- PostgreSQL database dump complete
--

\unrestrict JOxwLi7JScCzCxVUOOmZ8S33AP3vXHZhxS7dlABqhi03ry4zjdcX3tfwvrqxWnb

--
-- Database "tiktok_auth" dump
--

--
-- PostgreSQL database dump
--

\restrict 79Gan9Slq2nDwmoAacUMjHU2FhpcwLJJrtkTvx7WGrhd43AvmCiYIhbQzaH07gi

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_auth; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_auth WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_auth OWNER TO postgres;

\unrestrict 79Gan9Slq2nDwmoAacUMjHU2FhpcwLJJrtkTvx7WGrhd43AvmCiYIhbQzaH07gi
\connect tiktok_auth
\restrict 79Gan9Slq2nDwmoAacUMjHU2FhpcwLJJrtkTvx7WGrhd43AvmCiYIhbQzaH07gi

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: auth_provider; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.auth_provider AS ENUM (
    'local',
    'google',
    'apple'
);


ALTER TYPE public.auth_provider OWNER TO postgres;

--
-- Name: otp_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.otp_type AS ENUM (
    'phone_verification',
    'email_verification',
    'password_reset',
    'login'
);


ALTER TYPE public.otp_type OWNER TO postgres;

--
-- Name: user_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.user_status AS ENUM (
    'active',
    'inactive',
    'suspended',
    'deleted'
);


ALTER TYPE public.user_status OWNER TO postgres;

--
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
        NEW.updated_at = NOW();
        RETURN NEW;
    END;
    $$;


ALTER FUNCTION public.set_updated_at() OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: device_sessions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.device_sessions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    device_id text NOT NULL,
    device_name text DEFAULT ''::text NOT NULL,
    platform text DEFAULT ''::text NOT NULL,
    ip_address text DEFAULT ''::text NOT NULL,
    user_agent text DEFAULT ''::text NOT NULL,
    is_trusted boolean DEFAULT false NOT NULL,
    last_active_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone
);


ALTER TABLE public.device_sessions OWNER TO postgres;

--
-- Name: otp_codes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.otp_codes (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    code text NOT NULL,
    type public.otp_type NOT NULL,
    target text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_trials integer DEFAULT 5 NOT NULL,
    is_used boolean DEFAULT false NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.otp_codes OWNER TO postgres;

--
-- Name: sessions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.sessions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    refresh_token text NOT NULL,
    user_agent text DEFAULT ''::text NOT NULL,
    ip_address text DEFAULT ''::text NOT NULL,
    device_id text,
    is_revoked boolean DEFAULT false NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone
);


ALTER TABLE public.sessions OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email text,
    phone text,
    username text NOT NULL,
    password_hash text,
    provider public.auth_provider DEFAULT 'local'::public.auth_provider NOT NULL,
    provider_user_id text,
    email_verified boolean DEFAULT false NOT NULL,
    phone_verified boolean DEFAULT false NOT NULL,
    mfa_enabled boolean DEFAULT false NOT NULL,
    mfa_secret text,
    status public.user_status DEFAULT 'active'::public.user_status NOT NULL,
    display_name text,
    avatar_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.users OWNER TO postgres;

--
-- Data for Name: device_sessions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.device_sessions (id, user_id, device_id, device_name, platform, ip_address, user_agent, is_trusted, last_active_at, created_at, revoked_at) FROM stdin;
\.


--
-- Data for Name: otp_codes; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.otp_codes (id, user_id, code, type, target, attempts, max_trials, is_used, expires_at, created_at) FROM stdin;
\.


--
-- Data for Name: sessions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.sessions (id, user_id, refresh_token, user_agent, ip_address, device_id, is_revoked, expires_at, created_at, last_seen_at, revoked_at) FROM stdin;
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.users (id, email, phone, username, password_hash, provider, provider_user_id, email_verified, phone_verified, mfa_enabled, mfa_secret, status, display_name, avatar_url, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Name: device_sessions device_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_sessions
    ADD CONSTRAINT device_sessions_pkey PRIMARY KEY (id);


--
-- Name: otp_codes otp_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.otp_codes
    ADD CONSTRAINT otp_codes_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_refresh_token_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_refresh_token_key UNIQUE (refresh_token);


--
-- Name: device_sessions uq_device_user_device; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_sessions
    ADD CONSTRAINT uq_device_user_device UNIQUE (user_id, device_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: idx_otp_user_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_otp_user_type ON public.otp_codes USING btree (user_id, type) WHERE (NOT is_used);


--
-- Name: idx_sessions_expires_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_sessions_expires_at ON public.sessions USING btree (expires_at) WHERE (NOT is_revoked);


--
-- Name: idx_sessions_refresh_token; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_sessions_refresh_token ON public.sessions USING btree (refresh_token);


--
-- Name: idx_sessions_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_sessions_user_id ON public.sessions USING btree (user_id);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_email ON public.users USING btree (email) WHERE (deleted_at IS NULL);


--
-- Name: idx_users_phone; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_phone ON public.users USING btree (phone) WHERE (deleted_at IS NULL);


--
-- Name: idx_users_provider_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_users_provider_id ON public.users USING btree (provider, provider_user_id) WHERE (provider_user_id IS NOT NULL);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_username ON public.users USING btree (username) WHERE (deleted_at IS NULL);


--
-- Name: users trg_users_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: device_sessions device_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_sessions
    ADD CONSTRAINT device_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: otp_codes otp_codes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.otp_codes
    ADD CONSTRAINT otp_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict 79Gan9Slq2nDwmoAacUMjHU2FhpcwLJJrtkTvx7WGrhd43AvmCiYIhbQzaH07gi

--
-- Database "tiktok_comments" dump
--

--
-- PostgreSQL database dump
--

\restrict wqIq5WET7X9DKBOluyFnOQuEs3QyzAH7JM3aIWrg2Ob5Pv2JU2jPUJyqoc6knrT

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_comments; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_comments WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_comments OWNER TO postgres;

\unrestrict wqIq5WET7X9DKBOluyFnOQuEs3QyzAH7JM3aIWrg2Ob5Pv2JU2jPUJyqoc6knrT
\connect tiktok_comments
\restrict wqIq5WET7X9DKBOluyFnOQuEs3QyzAH7JM3aIWrg2Ob5Pv2JU2jPUJyqoc6knrT

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: comments; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.comments (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    video_id uuid NOT NULL,
    user_id uuid NOT NULL,
    parent_comment_id uuid,
    content text NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    reply_count integer DEFAULT 0 NOT NULL,
    is_deleted boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.comments OWNER TO postgres;

--
-- Data for Name: comments; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.comments (id, video_id, user_id, parent_comment_id, content, like_count, reply_count, is_deleted, created_at, updated_at) FROM stdin;
\.


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- Name: idx_comments_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_comments_user_id ON public.comments USING btree (user_id) WHERE (NOT is_deleted);


--
-- Name: idx_comments_video_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_comments_video_id ON public.comments USING btree (video_id, created_at DESC) WHERE (NOT is_deleted);


--
-- Name: comments comments_parent_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_parent_comment_id_fkey FOREIGN KEY (parent_comment_id) REFERENCES public.comments(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict wqIq5WET7X9DKBOluyFnOQuEs3QyzAH7JM3aIWrg2Ob5Pv2JU2jPUJyqoc6knrT

--
-- Database "tiktok_ecommerce" dump
--

--
-- PostgreSQL database dump
--

\restrict bZbbNNMauRxNQdugsgQc0BamOhIif1PRlckSPsd49J39Vct8xkbZrnufhCJZux2

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_ecommerce; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_ecommerce WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_ecommerce OWNER TO postgres;

\unrestrict bZbbNNMauRxNQdugsgQc0BamOhIif1PRlckSPsd49J39Vct8xkbZrnufhCJZux2
\connect tiktok_ecommerce
\restrict bZbbNNMauRxNQdugsgQc0BamOhIif1PRlckSPsd49J39Vct8xkbZrnufhCJZux2

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: order_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.order_status AS ENUM (
    'pending',
    'confirmed',
    'processing',
    'shipped',
    'delivered',
    'cancelled',
    'refunded'
);


ALTER TYPE public.order_status OWNER TO postgres;

--
-- Name: product_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.product_status AS ENUM (
    'draft',
    'active',
    'inactive',
    'deleted'
);


ALTER TYPE public.product_status OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: cart_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.cart_items (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cart_items_quantity_check CHECK ((quantity > 0))
);


ALTER TABLE public.cart_items OWNER TO postgres;

--
-- Name: order_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.order_items (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    order_id uuid NOT NULL,
    product_id uuid NOT NULL,
    seller_id uuid NOT NULL,
    quantity integer NOT NULL,
    unit_price numeric(12,2) NOT NULL,
    total_price numeric(12,2) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT order_items_quantity_check CHECK ((quantity > 0))
);


ALTER TABLE public.order_items OWNER TO postgres;

--
-- Name: orders; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.orders (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    buyer_id uuid NOT NULL,
    status public.order_status DEFAULT 'pending'::public.order_status NOT NULL,
    total_amount numeric(12,2) NOT NULL,
    currency text DEFAULT 'USD'::text NOT NULL,
    shipping_addr jsonb,
    tracking_number text,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.orders OWNER TO postgres;

--
-- Name: product_reviews; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.product_reviews (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    product_id uuid NOT NULL,
    user_id uuid NOT NULL,
    rating smallint NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    images text[] DEFAULT '{}'::text[] NOT NULL,
    is_verified boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT product_reviews_rating_check CHECK (((rating >= 1) AND (rating <= 5)))
);


ALTER TABLE public.product_reviews OWNER TO postgres;

--
-- Name: products; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.products (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    seller_id uuid NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    price numeric(12,2) NOT NULL,
    stock_quantity integer DEFAULT 0 NOT NULL,
    category text DEFAULT 'other'::text NOT NULL,
    images text[] DEFAULT '{}'::text[] NOT NULL,
    status public.product_status DEFAULT 'draft'::public.product_status NOT NULL,
    sku text,
    weight_kg numeric(8,3),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT products_price_check CHECK ((price >= (0)::numeric)),
    CONSTRAINT products_stock_quantity_check CHECK ((stock_quantity >= 0))
);


ALTER TABLE public.products OWNER TO postgres;

--
-- Data for Name: cart_items; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.cart_items (id, user_id, product_id, quantity, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: order_items; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.order_items (id, order_id, product_id, seller_id, quantity, unit_price, total_price, created_at) FROM stdin;
\.


--
-- Data for Name: orders; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.orders (id, buyer_id, status, total_amount, currency, shipping_addr, tracking_number, notes, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: product_reviews; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.product_reviews (id, product_id, user_id, rating, content, images, is_verified, created_at) FROM stdin;
\.


--
-- Data for Name: products; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.products (id, seller_id, name, description, price, stock_quantity, category, images, status, sku, weight_kg, created_at, updated_at) FROM stdin;
\.


--
-- Name: cart_items cart_items_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_pkey PRIMARY KEY (id);


--
-- Name: cart_items cart_items_user_id_product_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_user_id_product_id_key UNIQUE (user_id, product_id);


--
-- Name: order_items order_items_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_pkey PRIMARY KEY (id);


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


--
-- Name: product_reviews product_reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.product_reviews
    ADD CONSTRAINT product_reviews_pkey PRIMARY KEY (id);


--
-- Name: product_reviews product_reviews_product_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.product_reviews
    ADD CONSTRAINT product_reviews_product_id_user_id_key UNIQUE (product_id, user_id);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);


--
-- Name: products products_sku_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_sku_key UNIQUE (sku);


--
-- Name: idx_cart_items_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_cart_items_user_id ON public.cart_items USING btree (user_id);


--
-- Name: idx_order_items_order_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_order_items_order_id ON public.order_items USING btree (order_id);


--
-- Name: idx_order_items_seller_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_order_items_seller_id ON public.order_items USING btree (seller_id);


--
-- Name: idx_orders_buyer_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_orders_buyer_id ON public.orders USING btree (buyer_id, created_at DESC);


--
-- Name: idx_orders_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_orders_status ON public.orders USING btree (status);


--
-- Name: idx_product_reviews_product; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_product_reviews_product ON public.product_reviews USING btree (product_id, created_at DESC);


--
-- Name: idx_products_category; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_products_category ON public.products USING btree (category) WHERE (status = 'active'::public.product_status);


--
-- Name: idx_products_price; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_products_price ON public.products USING btree (price) WHERE (status = 'active'::public.product_status);


--
-- Name: idx_products_seller_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_products_seller_id ON public.products USING btree (seller_id) WHERE (status <> 'deleted'::public.product_status);


--
-- Name: cart_items cart_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id);


--
-- Name: order_items order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: order_items order_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id);


--
-- Name: product_reviews product_reviews_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.product_reviews
    ADD CONSTRAINT product_reviews_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict bZbbNNMauRxNQdugsgQc0BamOhIif1PRlckSPsd49J39Vct8xkbZrnufhCJZux2

--
-- Database "tiktok_feed" dump
--

--
-- PostgreSQL database dump
--

\restrict fQJlST7vkvIOwp78xRnbWH1tFmhejNn7WVAJLztsDgeMYwjO96vDxIurXi6LgAh

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_feed; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_feed WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_feed OWNER TO postgres;

\unrestrict fQJlST7vkvIOwp78xRnbWH1tFmhejNn7WVAJLztsDgeMYwjO96vDxIurXi6LgAh
\connect tiktok_feed
\restrict fQJlST7vkvIOwp78xRnbWH1tFmhejNn7WVAJLztsDgeMYwjO96vDxIurXi6LgAh

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: feed_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.feed_items (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    video_id uuid NOT NULL,
    score double precision DEFAULT 0 NOT NULL,
    feed_type text DEFAULT 'fyp'::text NOT NULL,
    expires_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.feed_items OWNER TO postgres;

--
-- Name: trending_videos; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.trending_videos (
    video_id uuid NOT NULL,
    trend_score double precision DEFAULT 0 NOT NULL,
    region text DEFAULT 'global'::text NOT NULL,
    category text,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.trending_videos OWNER TO postgres;

--
-- Data for Name: feed_items; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.feed_items (id, user_id, video_id, score, feed_type, expires_at, created_at) FROM stdin;
\.


--
-- Data for Name: trending_videos; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.trending_videos (video_id, trend_score, region, category, updated_at) FROM stdin;
\.


--
-- Name: feed_items feed_items_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feed_items
    ADD CONSTRAINT feed_items_pkey PRIMARY KEY (id);


--
-- Name: feed_items feed_items_user_id_video_id_feed_type_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.feed_items
    ADD CONSTRAINT feed_items_user_id_video_id_feed_type_key UNIQUE (user_id, video_id, feed_type);


--
-- Name: trending_videos trending_videos_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.trending_videos
    ADD CONSTRAINT trending_videos_pkey PRIMARY KEY (video_id);


--
-- Name: idx_feed_items_expires_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_feed_items_expires_at ON public.feed_items USING btree (expires_at) WHERE (expires_at IS NOT NULL);


--
-- Name: idx_feed_items_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_feed_items_user_id ON public.feed_items USING btree (user_id, feed_type, score DESC);


--
-- Name: idx_trending_region; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_trending_region ON public.trending_videos USING btree (region, trend_score DESC);


--
-- PostgreSQL database dump complete
--

\unrestrict fQJlST7vkvIOwp78xRnbWH1tFmhejNn7WVAJLztsDgeMYwjO96vDxIurXi6LgAh

--
-- Database "tiktok_interaction" dump
--

--
-- PostgreSQL database dump
--

\restrict XXf9blJ7ePi5ntN7lfR9tWbx6UhYWMvbdvO0Re7pG7HBHwSri1h4iO8hFzTIyeg

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_interaction; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_interaction WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_interaction OWNER TO postgres;

\unrestrict XXf9blJ7ePi5ntN7lfR9tWbx6UhYWMvbdvO0Re7pG7HBHwSri1h4iO8hFzTIyeg
\connect tiktok_interaction
\restrict XXf9blJ7ePi5ntN7lfR9tWbx6UhYWMvbdvO0Re7pG7HBHwSri1h4iO8hFzTIyeg

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: bookmarks; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.bookmarks (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    video_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.bookmarks OWNER TO postgres;

--
-- Name: comments; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.comments (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    video_id uuid NOT NULL,
    user_id uuid NOT NULL,
    parent_comment_id uuid,
    content text NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    reply_count integer DEFAULT 0 NOT NULL,
    is_pinned boolean DEFAULT false NOT NULL,
    is_deleted boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.comments OWNER TO postgres;

--
-- Name: likes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.likes (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    target_id uuid NOT NULL,
    target_type text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.likes OWNER TO postgres;

--
-- Name: video_views; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.video_views (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    video_id uuid NOT NULL,
    user_id uuid,
    watch_ms bigint DEFAULT 0 NOT NULL,
    source text DEFAULT 'fyp'::text NOT NULL,
    device_type text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.video_views OWNER TO postgres;

--
-- Data for Name: bookmarks; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.bookmarks (id, user_id, video_id, created_at) FROM stdin;
\.


--
-- Data for Name: comments; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.comments (id, video_id, user_id, parent_comment_id, content, like_count, reply_count, is_pinned, is_deleted, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: likes; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.likes (id, user_id, target_id, target_type, created_at) FROM stdin;
\.


--
-- Data for Name: video_views; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.video_views (id, video_id, user_id, watch_ms, source, device_type, created_at) FROM stdin;
\.


--
-- Name: bookmarks bookmarks_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_pkey PRIMARY KEY (id);


--
-- Name: bookmarks bookmarks_user_id_video_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_user_id_video_id_key UNIQUE (user_id, video_id);


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- Name: likes likes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.likes
    ADD CONSTRAINT likes_pkey PRIMARY KEY (id);


--
-- Name: likes likes_user_id_target_id_target_type_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.likes
    ADD CONSTRAINT likes_user_id_target_id_target_type_key UNIQUE (user_id, target_id, target_type);


--
-- Name: video_views video_views_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.video_views
    ADD CONSTRAINT video_views_pkey PRIMARY KEY (id);


--
-- Name: idx_bookmarks_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_bookmarks_user_id ON public.bookmarks USING btree (user_id, created_at DESC);


--
-- Name: idx_comments_parent_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_comments_parent_id ON public.comments USING btree (parent_comment_id) WHERE (parent_comment_id IS NOT NULL);


--
-- Name: idx_comments_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_comments_user_id ON public.comments USING btree (user_id) WHERE (NOT is_deleted);


--
-- Name: idx_comments_video_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_comments_video_id ON public.comments USING btree (video_id, created_at DESC) WHERE (NOT is_deleted);


--
-- Name: idx_likes_target; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_likes_target ON public.likes USING btree (target_id, target_type);


--
-- Name: idx_likes_user; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_likes_user ON public.likes USING btree (user_id);


--
-- Name: idx_video_views_video_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_video_views_video_id ON public.video_views USING btree (video_id, created_at DESC);


--
-- Name: comments comments_parent_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.comments
    ADD CONSTRAINT comments_parent_comment_id_fkey FOREIGN KEY (parent_comment_id) REFERENCES public.comments(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--

\unrestrict XXf9blJ7ePi5ntN7lfR9tWbx6UhYWMvbdvO0Re7pG7HBHwSri1h4iO8hFzTIyeg

--
-- Database "tiktok_likes" dump
--

--
-- PostgreSQL database dump
--

\restrict FqdCCV4Bo6RgJgGgADGQNx2rAlnVXkwfekNl4BxE1Ve2GrfghmMVm1wfYSqwcU8

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_likes; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_likes WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_likes OWNER TO postgres;

\unrestrict FqdCCV4Bo6RgJgGgADGQNx2rAlnVXkwfekNl4BxE1Ve2GrfghmMVm1wfYSqwcU8
\connect tiktok_likes
\restrict FqdCCV4Bo6RgJgGgADGQNx2rAlnVXkwfekNl4BxE1Ve2GrfghmMVm1wfYSqwcU8

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: likes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.likes (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    target_id uuid NOT NULL,
    target_type text DEFAULT 'video'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.likes OWNER TO postgres;

--
-- Data for Name: likes; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.likes (id, user_id, target_id, target_type, created_at) FROM stdin;
\.


--
-- Name: likes likes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.likes
    ADD CONSTRAINT likes_pkey PRIMARY KEY (id);


--
-- Name: likes likes_user_id_target_id_target_type_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.likes
    ADD CONSTRAINT likes_user_id_target_id_target_type_key UNIQUE (user_id, target_id, target_type);


--
-- Name: idx_likes_target; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_likes_target ON public.likes USING btree (target_id, target_type);


--
-- Name: idx_likes_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_likes_user_id ON public.likes USING btree (user_id, created_at DESC);


--
-- PostgreSQL database dump complete
--

\unrestrict FqdCCV4Bo6RgJgGgADGQNx2rAlnVXkwfekNl4BxE1Ve2GrfghmMVm1wfYSqwcU8

--
-- Database "tiktok_livestream" dump
--

--
-- PostgreSQL database dump
--

\restrict d7wPAT8hLFLUq8YrCw7QODYFoyL1MVuev74AaxpZy46Blukc7hLwWrk6QtreOsX

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_livestream; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_livestream WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_livestream OWNER TO postgres;

\unrestrict d7wPAT8hLFLUq8YrCw7QODYFoyL1MVuev74AaxpZy46Blukc7hLwWrk6QtreOsX
\connect tiktok_livestream
\restrict d7wPAT8hLFLUq8YrCw7QODYFoyL1MVuev74AaxpZy46Blukc7hLwWrk6QtreOsX

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: stream_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.stream_status AS ENUM (
    'scheduled',
    'live',
    'ended',
    'cancelled'
);


ALTER TYPE public.stream_status OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: stream_chat_messages; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.stream_chat_messages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    stream_id uuid NOT NULL,
    user_id uuid NOT NULL,
    content text NOT NULL,
    type text DEFAULT 'message'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.stream_chat_messages OWNER TO postgres;

--
-- Name: stream_gifts; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.stream_gifts (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    stream_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    gift_type text NOT NULL,
    gift_name text NOT NULL,
    coin_value integer NOT NULL,
    quantity integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.stream_gifts OWNER TO postgres;

--
-- Name: streams; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.streams (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    thumbnail_url text,
    stream_key text DEFAULT md5((random())::text) NOT NULL,
    rtmp_url text,
    hls_url text,
    status public.stream_status DEFAULT 'scheduled'::public.stream_status NOT NULL,
    viewer_count integer DEFAULT 0 NOT NULL,
    peak_viewers integer DEFAULT 0 NOT NULL,
    total_viewers integer DEFAULT 0 NOT NULL,
    gift_count integer DEFAULT 0 NOT NULL,
    coins_earned bigint DEFAULT 0 NOT NULL,
    category text,
    language text DEFAULT 'en'::text NOT NULL,
    is_recorded boolean DEFAULT false NOT NULL,
    recording_url text,
    scheduled_at timestamp with time zone,
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.streams OWNER TO postgres;

--
-- Data for Name: stream_chat_messages; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.stream_chat_messages (id, stream_id, user_id, content, type, created_at) FROM stdin;
\.


--
-- Data for Name: stream_gifts; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.stream_gifts (id, stream_id, sender_id, gift_type, gift_name, coin_value, quantity, created_at) FROM stdin;
\.


--
-- Data for Name: streams; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.streams (id, user_id, title, description, thumbnail_url, stream_key, rtmp_url, hls_url, status, viewer_count, peak_viewers, total_viewers, gift_count, coins_earned, category, language, is_recorded, recording_url, scheduled_at, started_at, ended_at, created_at, updated_at) FROM stdin;
\.


--
-- Name: stream_chat_messages stream_chat_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.stream_chat_messages
    ADD CONSTRAINT stream_chat_messages_pkey PRIMARY KEY (id);


--
-- Name: stream_gifts stream_gifts_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.stream_gifts
    ADD CONSTRAINT stream_gifts_pkey PRIMARY KEY (id);


--
-- Name: streams streams_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.streams
    ADD CONSTRAINT streams_pkey PRIMARY KEY (id);


--
-- Name: streams streams_stream_key_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.streams
    ADD CONSTRAINT streams_stream_key_key UNIQUE (stream_key);


--
-- Name: idx_stream_chat_stream_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_stream_chat_stream_id ON public.stream_chat_messages USING btree (stream_id, created_at DESC);


--
-- Name: idx_stream_gifts_stream_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_stream_gifts_stream_id ON public.stream_gifts USING btree (stream_id, created_at DESC);


--
-- Name: idx_streams_started_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_streams_started_at ON public.streams USING btree (started_at DESC);


--
-- Name: idx_streams_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_streams_status ON public.streams USING btree (status) WHERE (status = 'live'::public.stream_status);


--
-- Name: idx_streams_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_streams_user_id ON public.streams USING btree (user_id);


--
-- Name: stream_chat_messages stream_chat_messages_stream_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.stream_chat_messages
    ADD CONSTRAINT stream_chat_messages_stream_id_fkey FOREIGN KEY (stream_id) REFERENCES public.streams(id) ON DELETE CASCADE;


--
-- Name: stream_gifts stream_gifts_stream_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.stream_gifts
    ADD CONSTRAINT stream_gifts_stream_id_fkey FOREIGN KEY (stream_id) REFERENCES public.streams(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict d7wPAT8hLFLUq8YrCw7QODYFoyL1MVuev74AaxpZy46Blukc7hLwWrk6QtreOsX

--
-- Database "tiktok_messaging" dump
--

--
-- PostgreSQL database dump
--

\restrict kQOrf6azohMKheRIdBedopgc88ZGcI9dpuqmRdG2jWRzNV0iXFWPMnStbmtQ9hI

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_messaging; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_messaging WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_messaging OWNER TO postgres;

\unrestrict kQOrf6azohMKheRIdBedopgc88ZGcI9dpuqmRdG2jWRzNV0iXFWPMnStbmtQ9hI
\connect tiktok_messaging
\restrict kQOrf6azohMKheRIdBedopgc88ZGcI9dpuqmRdG2jWRzNV0iXFWPMnStbmtQ9hI

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: conversation_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.conversation_type AS ENUM (
    'direct',
    'group'
);


ALTER TYPE public.conversation_type OWNER TO postgres;

--
-- Name: message_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.message_type AS ENUM (
    'text',
    'image',
    'video',
    'audio',
    'file',
    'video_share',
    'system'
);


ALTER TYPE public.message_type OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: conversation_members; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.conversation_members (
    conversation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role text DEFAULT 'member'::text NOT NULL,
    joined_at timestamp with time zone DEFAULT now() NOT NULL,
    last_read_at timestamp with time zone,
    is_muted boolean DEFAULT false NOT NULL
);


ALTER TABLE public.conversation_members OWNER TO postgres;

--
-- Name: conversations; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.conversations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    type public.conversation_type DEFAULT 'direct'::public.conversation_type NOT NULL,
    name text,
    avatar_url text,
    last_msg_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.conversations OWNER TO postgres;

--
-- Name: messages; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.messages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    conversation_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    type public.message_type DEFAULT 'text'::public.message_type NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    media_url text,
    is_deleted boolean DEFAULT false NOT NULL,
    reply_to_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.messages OWNER TO postgres;

--
-- Data for Name: conversation_members; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.conversation_members (conversation_id, user_id, role, joined_at, last_read_at, is_muted) FROM stdin;
\.


--
-- Data for Name: conversations; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.conversations (id, type, name, avatar_url, last_msg_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: messages; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.messages (id, conversation_id, sender_id, type, content, media_url, is_deleted, reply_to_id, created_at, updated_at) FROM stdin;
\.


--
-- Name: conversation_members conversation_members_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.conversation_members
    ADD CONSTRAINT conversation_members_pkey PRIMARY KEY (conversation_id, user_id);


--
-- Name: conversations conversations_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.conversations
    ADD CONSTRAINT conversations_pkey PRIMARY KEY (id);


--
-- Name: messages messages_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_pkey PRIMARY KEY (id);


--
-- Name: idx_conv_members_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_conv_members_user_id ON public.conversation_members USING btree (user_id, joined_at DESC);


--
-- Name: idx_messages_conv_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_messages_conv_id ON public.messages USING btree (conversation_id, created_at DESC) WHERE (NOT is_deleted);


--
-- Name: idx_messages_sender_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_messages_sender_id ON public.messages USING btree (sender_id);


--
-- Name: conversation_members conversation_members_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.conversation_members
    ADD CONSTRAINT conversation_members_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: messages messages_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: messages messages_reply_to_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_reply_to_id_fkey FOREIGN KEY (reply_to_id) REFERENCES public.messages(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--

\unrestrict kQOrf6azohMKheRIdBedopgc88ZGcI9dpuqmRdG2jWRzNV0iXFWPMnStbmtQ9hI

--
-- Database "tiktok_notifications" dump
--

--
-- PostgreSQL database dump
--

\restrict yGRssbGeWoeDYlZ4RSRrAn4YdrazSqQ7G9d6k13I1ohjNfN2Ejz2fXsG8c46My3

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_notifications; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_notifications WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_notifications OWNER TO postgres;

\unrestrict yGRssbGeWoeDYlZ4RSRrAn4YdrazSqQ7G9d6k13I1ohjNfN2Ejz2fXsG8c46My3
\connect tiktok_notifications
\restrict yGRssbGeWoeDYlZ4RSRrAn4YdrazSqQ7G9d6k13I1ohjNfN2Ejz2fXsG8c46My3

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: notification_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.notification_type AS ENUM (
    'like',
    'comment',
    'follow',
    'mention',
    'gift',
    'live_start',
    'order_update',
    'system'
);


ALTER TYPE public.notification_type OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: device_tokens; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.device_tokens (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    token text NOT NULL,
    platform text NOT NULL,
    app_version text,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.device_tokens OWNER TO postgres;

--
-- Name: notification_preferences; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.notification_preferences (
    user_id uuid NOT NULL,
    push_enabled boolean DEFAULT true NOT NULL,
    email_enabled boolean DEFAULT true NOT NULL,
    likes_enabled boolean DEFAULT true NOT NULL,
    comments_enabled boolean DEFAULT true NOT NULL,
    follows_enabled boolean DEFAULT true NOT NULL,
    mentions_enabled boolean DEFAULT true NOT NULL,
    gifts_enabled boolean DEFAULT true NOT NULL,
    live_enabled boolean DEFAULT true NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.notification_preferences OWNER TO postgres;

--
-- Name: notifications; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.notifications (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    type public.notification_type NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    data jsonb,
    actor_id uuid,
    entity_id uuid,
    entity_type text,
    is_read boolean DEFAULT false NOT NULL,
    read_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.notifications OWNER TO postgres;

--
-- Data for Name: device_tokens; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.device_tokens (id, user_id, token, platform, app_version, is_active, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: notification_preferences; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.notification_preferences (user_id, push_enabled, email_enabled, likes_enabled, comments_enabled, follows_enabled, mentions_enabled, gifts_enabled, live_enabled, updated_at) FROM stdin;
\.


--
-- Data for Name: notifications; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.notifications (id, user_id, type, title, body, data, actor_id, entity_id, entity_type, is_read, read_at, created_at) FROM stdin;
\.


--
-- Name: device_tokens device_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_tokens
    ADD CONSTRAINT device_tokens_pkey PRIMARY KEY (id);


--
-- Name: device_tokens device_tokens_token_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_tokens
    ADD CONSTRAINT device_tokens_token_key UNIQUE (token);


--
-- Name: notification_preferences notification_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_preferences
    ADD CONSTRAINT notification_preferences_pkey PRIMARY KEY (user_id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: idx_device_tokens_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_device_tokens_user_id ON public.device_tokens USING btree (user_id) WHERE is_active;


--
-- Name: idx_notifications_unread; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_notifications_unread ON public.notifications USING btree (user_id, is_read) WHERE (NOT is_read);


--
-- Name: idx_notifications_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_notifications_user_id ON public.notifications USING btree (user_id, created_at DESC);


--
-- PostgreSQL database dump complete
--

\unrestrict yGRssbGeWoeDYlZ4RSRrAn4YdrazSqQ7G9d6k13I1ohjNfN2Ejz2fXsG8c46My3

--
-- Database "tiktok_recommendations" dump
--

--
-- PostgreSQL database dump
--

\restrict Qs4XV5r9s4Ky2rnyrXLu2qBvmp64fim0cxqJb0pbitLtdPEfRaJYJFKoJNqExEI

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_recommendations; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_recommendations WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_recommendations OWNER TO postgres;

\unrestrict Qs4XV5r9s4Ky2rnyrXLu2qBvmp64fim0cxqJb0pbitLtdPEfRaJYJFKoJNqExEI
\connect tiktok_recommendations
\restrict Qs4XV5r9s4Ky2rnyrXLu2qBvmp64fim0cxqJb0pbitLtdPEfRaJYJFKoJNqExEI

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: content_scores; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.content_scores (
    user_id uuid NOT NULL,
    video_id uuid NOT NULL,
    score double precision DEFAULT 0 NOT NULL,
    reason text,
    expires_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.content_scores OWNER TO postgres;

--
-- Name: user_preferences; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_preferences (
    user_id uuid NOT NULL,
    preferred_categories text[] DEFAULT '{}'::text[] NOT NULL,
    preferred_sounds text[] DEFAULT '{}'::text[] NOT NULL,
    language text DEFAULT 'en'::text NOT NULL,
    algorithm_version text DEFAULT 'v1'::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.user_preferences OWNER TO postgres;

--
-- Data for Name: content_scores; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.content_scores (user_id, video_id, score, reason, expires_at, created_at) FROM stdin;
\.


--
-- Data for Name: user_preferences; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.user_preferences (user_id, preferred_categories, preferred_sounds, language, algorithm_version, updated_at) FROM stdin;
\.


--
-- Name: content_scores content_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.content_scores
    ADD CONSTRAINT content_scores_pkey PRIMARY KEY (user_id, video_id);


--
-- Name: user_preferences user_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (user_id);


--
-- Name: idx_content_scores_user_score; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_content_scores_user_score ON public.content_scores USING btree (user_id, score DESC);


--
-- PostgreSQL database dump complete
--

\unrestrict Qs4XV5r9s4Ky2rnyrXLu2qBvmp64fim0cxqJb0pbitLtdPEfRaJYJFKoJNqExEI

--
-- Database "tiktok_reports" dump
--

--
-- PostgreSQL database dump
--

\restrict ivNU34rzVPEfbbtVAFKnqdt68DJcrRRWdUcrhOPaGbo77GdvRFolNQXDOwfI3di

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_reports; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_reports WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_reports OWNER TO postgres;

\unrestrict ivNU34rzVPEfbbtVAFKnqdt68DJcrRRWdUcrhOPaGbo77GdvRFolNQXDOwfI3di
\connect tiktok_reports
\restrict ivNU34rzVPEfbbtVAFKnqdt68DJcrRRWdUcrhOPaGbo77GdvRFolNQXDOwfI3di

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: report_entity_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.report_entity_type AS ENUM (
    'video',
    'user',
    'comment',
    'live_stream',
    'message'
);


ALTER TYPE public.report_entity_type OWNER TO postgres;

--
-- Name: report_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.report_status AS ENUM (
    'pending',
    'reviewing',
    'resolved',
    'dismissed'
);


ALTER TYPE public.report_status OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: reports; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.reports (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    reporter_id uuid NOT NULL,
    entity_id uuid NOT NULL,
    entity_type public.report_entity_type NOT NULL,
    reason text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    status public.report_status DEFAULT 'pending'::public.report_status NOT NULL,
    reviewed_by uuid,
    review_notes text,
    resolved_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.reports OWNER TO postgres;

--
-- Data for Name: reports; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.reports (id, reporter_id, entity_id, entity_type, reason, description, status, reviewed_by, review_notes, resolved_at, created_at, updated_at) FROM stdin;
\.


--
-- Name: reports reports_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_pkey PRIMARY KEY (id);


--
-- Name: idx_reports_entity; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_reports_entity ON public.reports USING btree (entity_id, entity_type);


--
-- Name: idx_reports_reporter; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_reports_reporter ON public.reports USING btree (reporter_id);


--
-- Name: idx_reports_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_reports_status ON public.reports USING btree (status, created_at DESC);


--
-- PostgreSQL database dump complete
--

\unrestrict ivNU34rzVPEfbbtVAFKnqdt68DJcrRRWdUcrhOPaGbo77GdvRFolNQXDOwfI3di

--
-- Database "tiktok_social" dump
--

--
-- PostgreSQL database dump
--

\restrict 1OgwvKiYXY0fRTCSe1olke3jnnTzoSQQaFaxT2A0pGQTvebAtFSNSe4HVUhHmIh

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_social; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_social WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_social OWNER TO postgres;

\unrestrict 1OgwvKiYXY0fRTCSe1olke3jnnTzoSQQaFaxT2A0pGQTvebAtFSNSe4HVUhHmIh
\connect tiktok_social
\restrict 1OgwvKiYXY0fRTCSe1olke3jnnTzoSQQaFaxT2A0pGQTvebAtFSNSe4HVUhHmIh

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: blocked_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.blocked_users (
    blocker_id uuid NOT NULL,
    blocked_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.blocked_users OWNER TO postgres;

--
-- Name: follows; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.follows (
    follower_id uuid NOT NULL,
    following_id uuid NOT NULL,
    is_mutual boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.follows OWNER TO postgres;

--
-- Data for Name: blocked_users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.blocked_users (blocker_id, blocked_id, created_at) FROM stdin;
\.


--
-- Data for Name: follows; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.follows (follower_id, following_id, is_mutual, created_at) FROM stdin;
\.


--
-- Name: blocked_users blocked_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.blocked_users
    ADD CONSTRAINT blocked_users_pkey PRIMARY KEY (blocker_id, blocked_id);


--
-- Name: follows follows_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pkey PRIMARY KEY (follower_id, following_id);


--
-- Name: idx_blocked_users_blocker; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_blocked_users_blocker ON public.blocked_users USING btree (blocker_id);


--
-- Name: idx_follows_follower_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_follows_follower_id ON public.follows USING btree (follower_id, created_at DESC);


--
-- Name: idx_follows_following_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_follows_following_id ON public.follows USING btree (following_id, created_at DESC);


--
-- PostgreSQL database dump complete
--

\unrestrict 1OgwvKiYXY0fRTCSe1olke3jnnTzoSQQaFaxT2A0pGQTvebAtFSNSe4HVUhHmIh

--
-- Database "tiktok_users" dump
--

--
-- PostgreSQL database dump
--

\restrict fS8sHxSR4MOyfUji81aQggNRSaofejEYKHuPaxfCvG7tVo9efTi934MVaDow5S8

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_users; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_users WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_users OWNER TO postgres;

\unrestrict fS8sHxSR4MOyfUji81aQggNRSaofejEYKHuPaxfCvG7tVo9efTi934MVaDow5S8
\connect tiktok_users
\restrict fS8sHxSR4MOyfUji81aQggNRSaofejEYKHuPaxfCvG7tVo9efTi934MVaDow5S8

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: user_blocks; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_blocks (
    blocker_id uuid NOT NULL,
    blocked_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.user_blocks OWNER TO postgres;

--
-- Name: user_settings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_settings (
    user_id uuid NOT NULL,
    language text DEFAULT 'en'::text NOT NULL,
    notification_push boolean DEFAULT true NOT NULL,
    notification_email boolean DEFAULT true NOT NULL,
    allow_duet boolean DEFAULT true NOT NULL,
    allow_stitch boolean DEFAULT true NOT NULL,
    allow_download boolean DEFAULT true NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.user_settings OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    username public.citext NOT NULL,
    display_name text DEFAULT ''::text NOT NULL,
    email text,
    phone text,
    avatar_url text DEFAULT ''::text NOT NULL,
    bio text DEFAULT ''::text NOT NULL,
    website_url text,
    location text,
    is_verified boolean DEFAULT false NOT NULL,
    is_private boolean DEFAULT false NOT NULL,
    followers_count integer DEFAULT 0 NOT NULL,
    following_count integer DEFAULT 0 NOT NULL,
    videos_count integer DEFAULT 0 NOT NULL,
    likes_received bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


ALTER TABLE public.users OWNER TO postgres;

--
-- Data for Name: user_blocks; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.user_blocks (blocker_id, blocked_id, created_at) FROM stdin;
\.


--
-- Data for Name: user_settings; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.user_settings (user_id, language, notification_push, notification_email, allow_duet, allow_stitch, allow_download, updated_at) FROM stdin;
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.users (id, username, display_name, email, phone, avatar_url, bio, website_url, location, is_verified, is_private, followers_count, following_count, videos_count, likes_received, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Name: user_blocks user_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_blocks
    ADD CONSTRAINT user_blocks_pkey PRIMARY KEY (blocker_id, blocked_id);


--
-- Name: user_settings user_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_settings
    ADD CONSTRAINT user_settings_pkey PRIMARY KEY (user_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_email ON public.users USING btree (email) WHERE (deleted_at IS NULL);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_username ON public.users USING btree (username) WHERE (deleted_at IS NULL);


--
-- Name: user_blocks user_blocks_blocked_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_blocks
    ADD CONSTRAINT user_blocks_blocked_id_fkey FOREIGN KEY (blocked_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_blocks user_blocks_blocker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_blocks
    ADD CONSTRAINT user_blocks_blocker_id_fkey FOREIGN KEY (blocker_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_settings user_settings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_settings
    ADD CONSTRAINT user_settings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict fS8sHxSR4MOyfUji81aQggNRSaofejEYKHuPaxfCvG7tVo9efTi934MVaDow5S8

--
-- Database "tiktok_videos" dump
--

--
-- PostgreSQL database dump
--

\restrict rxQREmHy8MQt3fIxHQXxR4w4LsoLYK5sUgdOlcBhdDd9Wdt0rYMGn9hEEylILB6

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_videos; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_videos WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_videos OWNER TO postgres;

\unrestrict rxQREmHy8MQt3fIxHQXxR4w4LsoLYK5sUgdOlcBhdDd9Wdt0rYMGn9hEEylILB6
\connect tiktok_videos
\restrict rxQREmHy8MQt3fIxHQXxR4w4LsoLYK5sUgdOlcBhdDd9Wdt0rYMGn9hEEylILB6

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: video_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.video_status AS ENUM (
    'uploading',
    'processing',
    'published',
    'failed',
    'deleted',
    'private'
);


ALTER TYPE public.video_status OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: upload_sessions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.upload_sessions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    file_name text NOT NULL,
    file_size bigint NOT NULL,
    mime_type text NOT NULL,
    total_chunks integer NOT NULL,
    chunk_size bigint NOT NULL,
    uploaded_chunks integer[] DEFAULT '{}'::integer[] NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    storage_key text,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.upload_sessions OWNER TO postgres;

--
-- Name: video_hashtags; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.video_hashtags (
    video_id uuid NOT NULL,
    hashtag text NOT NULL
);


ALTER TABLE public.video_hashtags OWNER TO postgres;

--
-- Name: videos; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.videos (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    video_url text DEFAULT ''::text NOT NULL,
    hls_url text,
    thumbnail_url text,
    duration_ms bigint DEFAULT 0 NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    mime_type text DEFAULT 'video/mp4'::text NOT NULL,
    width integer,
    height integer,
    status public.video_status DEFAULT 'uploading'::public.video_status NOT NULL,
    view_count bigint DEFAULT 0 NOT NULL,
    like_count bigint DEFAULT 0 NOT NULL,
    comment_count bigint DEFAULT 0 NOT NULL,
    share_count bigint DEFAULT 0 NOT NULL,
    bookmark_count bigint DEFAULT 0 NOT NULL,
    is_private boolean DEFAULT false NOT NULL,
    allow_comments boolean DEFAULT true NOT NULL,
    allow_duet boolean DEFAULT true NOT NULL,
    allow_stitch boolean DEFAULT true NOT NULL,
    location text,
    language text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    deleted_at timestamp with time zone
);


ALTER TABLE public.videos OWNER TO postgres;

--
-- Data for Name: upload_sessions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.upload_sessions (id, user_id, file_name, file_size, mime_type, total_chunks, chunk_size, uploaded_chunks, status, storage_key, expires_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: video_hashtags; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.video_hashtags (video_id, hashtag) FROM stdin;
\.


--
-- Data for Name: videos; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.videos (id, user_id, title, description, video_url, hls_url, thumbnail_url, duration_ms, file_size, mime_type, width, height, status, view_count, like_count, comment_count, share_count, bookmark_count, is_private, allow_comments, allow_duet, allow_stitch, location, language, created_at, updated_at, published_at, deleted_at) FROM stdin;
\.


--
-- Name: upload_sessions upload_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.upload_sessions
    ADD CONSTRAINT upload_sessions_pkey PRIMARY KEY (id);


--
-- Name: video_hashtags video_hashtags_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.video_hashtags
    ADD CONSTRAINT video_hashtags_pkey PRIMARY KEY (video_id, hashtag);


--
-- Name: videos videos_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.videos
    ADD CONSTRAINT videos_pkey PRIMARY KEY (id);


--
-- Name: idx_upload_sessions_expires_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_upload_sessions_expires_at ON public.upload_sessions USING btree (expires_at);


--
-- Name: idx_upload_sessions_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_upload_sessions_user_id ON public.upload_sessions USING btree (user_id);


--
-- Name: idx_video_hashtags_tag; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_video_hashtags_tag ON public.video_hashtags USING btree (hashtag);


--
-- Name: idx_videos_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_videos_created_at ON public.videos USING btree (created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_videos_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_videos_status ON public.videos USING btree (status) WHERE (deleted_at IS NULL);


--
-- Name: idx_videos_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_videos_user_id ON public.videos USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: video_hashtags video_hashtags_video_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.video_hashtags
    ADD CONSTRAINT video_hashtags_video_id_fkey FOREIGN KEY (video_id) REFERENCES public.videos(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict rxQREmHy8MQt3fIxHQXxR4w4LsoLYK5sUgdOlcBhdDd9Wdt0rYMGn9hEEylILB6

--
-- Database "tiktok_wallet" dump
--

--
-- PostgreSQL database dump
--

\restrict bYMPVKW6NQLwMZmlQii3jzRh6UkIlG9rarmC3wHMMStMYASXyxeVCtyTeyrpMAm

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tiktok_wallet; Type: DATABASE; Schema: -; Owner: postgres
--

CREATE DATABASE tiktok_wallet WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'en_US.utf8';


ALTER DATABASE tiktok_wallet OWNER TO postgres;

\unrestrict bYMPVKW6NQLwMZmlQii3jzRh6UkIlG9rarmC3wHMMStMYASXyxeVCtyTeyrpMAm
\connect tiktok_wallet
\restrict bYMPVKW6NQLwMZmlQii3jzRh6UkIlG9rarmC3wHMMStMYASXyxeVCtyTeyrpMAm

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: transaction_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.transaction_status AS ENUM (
    'pending',
    'completed',
    'failed',
    'cancelled',
    'refunded'
);


ALTER TYPE public.transaction_status OWNER TO postgres;

--
-- Name: transaction_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.transaction_type AS ENUM (
    'deposit',
    'withdrawal',
    'transfer_in',
    'transfer_out',
    'gift_sent',
    'gift_received',
    'coin_purchase',
    'coin_convert',
    'refund'
);


ALTER TYPE public.transaction_type OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: coin_packages; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.coin_packages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name text NOT NULL,
    coins bigint NOT NULL,
    price_usd numeric(8,2) NOT NULL,
    bonus_coins bigint DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.coin_packages OWNER TO postgres;

--
-- Name: transactions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.transactions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    wallet_id uuid NOT NULL,
    type public.transaction_type NOT NULL,
    amount numeric(18,4) NOT NULL,
    currency text DEFAULT 'USD'::text NOT NULL,
    status public.transaction_status DEFAULT 'pending'::public.transaction_status NOT NULL,
    reference_id text,
    reference_type text,
    description text,
    metadata jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.transactions OWNER TO postgres;

--
-- Name: wallets; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.wallets (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    balance numeric(18,4) DEFAULT 0 NOT NULL,
    coin_balance bigint DEFAULT 0 NOT NULL,
    currency text DEFAULT 'USD'::text NOT NULL,
    is_frozen boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT wallets_balance_check CHECK ((balance >= (0)::numeric)),
    CONSTRAINT wallets_coin_balance_check CHECK ((coin_balance >= 0))
);


ALTER TABLE public.wallets OWNER TO postgres;

--
-- Data for Name: coin_packages; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.coin_packages (id, name, coins, price_usd, bonus_coins, is_active, created_at) FROM stdin;
ba4874ee-694c-4b99-833c-c42ffada6f68	Starter	100	0.99	0	t	2026-06-27 09:10:07.733352+00
9dc8eadb-68aa-481a-83d0-922f7fa75aff	Basic	500	4.99	0	t	2026-06-27 09:10:07.733352+00
b3c38df5-866e-424b-8902-2d97f65dc297	Popular	1000	9.99	0	t	2026-06-27 09:10:07.733352+00
031bcaf0-5135-4711-9b72-1e12fdf2c734	Value	2500	19.99	0	t	2026-06-27 09:10:07.733352+00
e59113ef-d9e6-4d49-ac9b-47522037a653	Premium	5000	34.99	0	t	2026-06-27 09:10:07.733352+00
e526510d-7446-4382-a7a1-7f6279c99b21	Ultimate	10000	64.99	0	t	2026-06-27 09:10:07.733352+00
\.


--
-- Data for Name: transactions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.transactions (id, wallet_id, type, amount, currency, status, reference_id, reference_type, description, metadata, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: wallets; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.wallets (id, user_id, balance, coin_balance, currency, is_frozen, created_at, updated_at) FROM stdin;
\.


--
-- Name: coin_packages coin_packages_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.coin_packages
    ADD CONSTRAINT coin_packages_pkey PRIMARY KEY (id);


--
-- Name: transactions transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: wallets wallets_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.wallets
    ADD CONSTRAINT wallets_pkey PRIMARY KEY (id);


--
-- Name: wallets wallets_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.wallets
    ADD CONSTRAINT wallets_user_id_key UNIQUE (user_id);


--
-- Name: idx_transactions_reference; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_transactions_reference ON public.transactions USING btree (reference_id, reference_type);


--
-- Name: idx_transactions_wallet_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_transactions_wallet_id ON public.transactions USING btree (wallet_id, created_at DESC);


--
-- Name: transactions transactions_wallet_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_wallet_id_fkey FOREIGN KEY (wallet_id) REFERENCES public.wallets(id);


--
-- PostgreSQL database dump complete
--

\unrestrict bYMPVKW6NQLwMZmlQii3jzRh6UkIlG9rarmC3wHMMStMYASXyxeVCtyTeyrpMAm

--
-- PostgreSQL database cluster dump complete
--

