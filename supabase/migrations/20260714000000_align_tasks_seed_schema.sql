DO $$
DECLARE
  constraint_name text;
BEGIN
  FOR constraint_name IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'public.tasks'::regclass
      AND contype = 'c'
      AND (pg_get_constraintdef(oid) ILIKE '%service_type%' OR pg_get_constraintdef(oid) ILIKE '%status%')
  LOOP
    EXECUTE format('ALTER TABLE public.tasks DROP CONSTRAINT %I', constraint_name);
  END LOOP;
END $$;

ALTER TABLE public.tasks
  ALTER COLUMN created_at TYPE text USING created_at::text;

ALTER TABLE public.tasks
  ADD CONSTRAINT tasks_service_type_check CHECK (service_type IN ('video_call', 'voice_call', 'phone_call', 'chat')),
  ADD CONSTRAINT tasks_status_check CHECK (status IN ('new', 'pending', 'in_progress', 'completed'));
