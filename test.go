

package main                                                                 
		

const tasksDB = "tasks.yaml"

func LoadTasks() objects.TaskStore {
	// Implementation for loading tasks

	data = os.ReadFile(tasksDB)
}

func cmdAdd(args []string) {
	// Implementation for adding a task
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("title: ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimpSpace(title)

	fmt.Print("description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	store := loadTasks()

}


func main() {                                                                
	switch os.Args[1] {
		case "add":
			cmdAdd()
		default:
			fmt.Fprintln("unknown command")
			os.Exit(1)
	}	
}                                              