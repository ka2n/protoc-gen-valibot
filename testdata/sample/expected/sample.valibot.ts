import { array, number, object, optional, string } from 'valibot';

export const TestMessageSchema = () => object({
	name: string(),
 	id: number(),
 	email: array(string()),
 	test_message_2: optional(TestMessage2Schema())
})

export const TestMessage2Schema = () => object({
	name: string()
})

